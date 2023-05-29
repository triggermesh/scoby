// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
	"github.com/triggermesh/scoby/pkg/utils/configmap"
	"github.com/triggermesh/scoby/pkg/utils/resolver"
	"github.com/triggermesh/scoby/pkg/utils/resources"
)

// rootObject where the renderer will start inspecting
const (
	rootObject = "spec"
)

type renderer struct {
	resolver resolver.Resolver

	// Global options to be applied while transforming object fields
	// into workload parameters.
	global commonv1alpha1.GlobalParameterConfiguration

	// Set of rules that add or fill elements at the object status.
	//
	// TODO maybe move to status renderer object
	addStatus []commonv1alpha1.StatusAddElement

	add  *addRenderer
	spec *specRenderer
}

// NewRenderer creates a new renderer object for reconciliation purposes.
// The renderer needs a workload definition to parse to apply the instructions contained in it on
// the incoming objects.
// The resolver is needed to parse objects into URIs at built in functions.
func NewRenderer(wkl *commonv1alpha1.Workload, resolver resolver.Resolver, cmr configmap.Reader) (reconciler.ObjectRenderer, error) {
	r := &renderer{
		resolver: resolver,
	}

	// Store at renderer a copy of the workload status configuration
	if wkl.StatusConfiguration != nil {
		scfg := wkl.StatusConfiguration

		if scfg.AddElements != nil && len(scfg.AddElements) != 0 {
			r.addStatus = make([]commonv1alpha1.StatusAddElement, len(scfg.AddElements))
			copy(r.addStatus, scfg.AddElements)
		}
	}

	pcfg := wkl.ParameterConfiguration
	if pcfg == nil {
		return r, nil
	}

	if pcfg.Global != nil {
		r.global = *pcfg.Global
	}

	add, err := newAddRenderer(pcfg.Add, cmr)
	if err != nil {
		return nil, err
	}

	r.add = add

	spec, err := newSpecRenderer(pcfg.FromSpec, cmr)
	if err != nil {
		return nil, err
	}

	r.spec = spec

	return r, nil
}

func (r *renderer) Render(ctx context.Context, obj reconciler.Object) error {
	uobj, ok := obj.AsKubeObject().(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("could not parse object into unstructured: %s", obj.GetName())
	}

	// not having a spec is possible, just return without error
	uobjRoot, ok := uobj.Object[rootObject]
	if !ok {
		return nil
	}

	root, ok := uobjRoot.(map[string]interface{})
	if !ok {
		return fmt.Errorf("object %q is expected to be a map[string]interface{}", rootObject)
	}

	// do a first pass of the unstructured and turn it into an
	// structure that can be used to apply the registered configuration.
	parsedFields := r.restructureIntoParsedFields(root, []string{rootObject})

	err := r.renderParsedFields(ctx, obj, parsedFields)
	if err != nil {
		return err
	}

	err = r.renderStatus(obj)
	if err != nil {
		return err
	}

	return nil
}

func (r *renderer) renderParsedFields(ctx context.Context, obj reconciler.Object, pfs parseFields) error {

	// Iterate default environment variables defined at the registration, if they are not
	// present at the object's parsed fields, add them now with the defaulted value.
	for k := range r.spec.evDefaultValuesByPath {
		if _, ok := pfs[k]; !ok {
			pfs[k] = parsedField{
				branch: strings.Split(k, "."),
				// The value will be set later when we iterate all
				// parsed fields.
				value: nil,
			}
		}
	}

	// Order parsed fields to be able to process elements that do custom rendering, and
	// that need to avoid processing of nested elements. Secret and ConfigMap rednering are
	// an example where:
	//
	// spec:
	//   mySecret:
	//     name: secret
	//     key: password
	//
	// Will expect to generate an environment variable for spec.mySecret, but not for
	// the inner elements.
	fieldNames := make([]string, 0, len(pfs))
	for fn := range pfs {
		fieldNames = append(fieldNames, fn)
	}

	sort.Strings(fieldNames)

	// Keep field name prefixes that should be avoided in this array.
	avoidFieldPrefixes := []string{}

	evs, err := r.add.renderEnvVars(ctx)
	if err != nil {
		return fmt.Errorf("rendering added envrionment variables: %w", err)
	}

	for path := range evs {
		obj.AddEnvVar(path, evs[path])
	}

	// Iterate all elements in the user object parsed fields structure.
	for _, k := range fieldNames {

		// Check if the field should be avoided. This works for nested items because
		// the fields are sorted, and nested elements are parsed after root objects
		// that might mark subpaths as non parseables.
		avoid := false
		for i := range avoidFieldPrefixes {
			if strings.HasPrefix(k, avoidFieldPrefixes[i]) {
				avoid = true
				break
			}
		}
		if avoid {
			continue
		}

		pf := pfs[k]
		path := pf.toJSONPath()

		// Check soon if the value needs to be skipped, move over to the next.
		if _, ok := r.spec.skipsByPath[path]; ok {
			continue
		}

		_, specInstructions := r.spec.allByPath[path]
		if !specInstructions {

			// Skip intermediate nodes that have no customizations, they exist only
			// to allow them to be used for calculations.
			if pf.intermediateNode {
				continue
			}
		}

		if refV, ok := r.spec.volumeByPath[path]; ok {
			v, err := pfs.volumeReferenceToVolume(&refV)
			if err != nil {
				return err
			}

			obj.AddVolumeMount(path, v)

			// Do not parse any internal elements at next iterations.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)
			continue
		}

		// From this point on rendering will generate an environment variable.

		// evName contains the environment variable name.
		evName := ""
		if v, ok := r.spec.evNameByPath[path]; ok {
			// Use the provided name of the environment variable when set at registration.
			evName = v
		} else {
			// Default name is the joined and uppercased element path.
			evName = strings.ToUpper(strings.Join(pf.branch[1:], "_"))

			// When the environment variable name is not explicitly set, check if
			// a global prefix exists.
			if prefix := r.global.GetDefaultPrefix(); prefix != "" {
				evName = prefix + evName
			}
		}

		// If there is no value provided at the user input and there is a
		// default value at registration, use it.
		if v, ok := r.spec.evDefaultValuesByPath[path]; ok && pf.value == nil {
			obj.AddEnvVar(path, v.ToEnv(evName))

			// Do not parse any internal elements at next iterations.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)
			continue
		}

		if v, ok := r.spec.evConfigMapByPath[path]; ok {
			evs, err := pfs.configMapReferenceToEnvVarSource(&v)
			if err != nil {
				return err
			}

			obj.AddEnvVar(path,
				&corev1.EnvVar{
					Name:      evName,
					ValueFrom: evs,
				},
			)

			// Do not parse any internal elements at next iterations.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)
			continue
		}

		if v, ok := r.spec.evSecretByPath[path]; ok {
			evs, err := pfs.secretReferenceToEnvVarSource(&v)
			if err != nil {
				return err
			}

			obj.AddEnvVar(path,
				&corev1.EnvVar{
					Name:      evName,
					ValueFrom: evs,
				},
			)

			// Do not parse any internal elements at next iterations.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)
			continue
		}

		if v, ok := r.spec.evBuiltInFunctionByPath[path]; ok {
			switch v.Name {
			case "resolveAddress":
				ev, err := r.builtInResolveAddress(ctx, &pf, obj.GetNamespace(), evName)
				if err != nil {
					return fmt.Errorf("could not resolve address at %s: %w", k, err)
				}

				obj.AddEnvVar(path, ev)

				// Do not parse any internal elements at next iterations.
				avoidFieldPrefixes = append(avoidFieldPrefixes, k)
				continue

			}
			// Do not parse any internal elements at next iterations.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)
			continue
		}

		// There are no workload configuration rules, fallback to default rendering
		ev, err := r.defaultRendering(&pf, evName)
		if err != nil {
			return fmt.Errorf("could not apply default rendering at %q: %w", k, err)
		}

		obj.AddEnvVar(path, ev)
	}

	return nil
}

// parsedField is a representation of a user instance element
// containing the location, value, and in case of arrays the
// parsedFields under it.
type parsedField struct {
	// string array depicting the element's hierarchy.
	branch []string

	// raw value found under this element.
	value interface{}

	// when the element is an array this field
	// contains each parsedField. This structure makes
	// it easy to extend parsing capabilities using
	// references to items in arrays.
	array map[string]parsedField

	// intermediate nodes are those maps that have sub nodes,
	// and that we store to be able to use them when rendering
	// if needed.
	intermediateNode bool
}

// toJSONPath is a JSON
func (v *parsedField) toJSONPath() string {
	return strings.Join(v.branch, ".")
}

type parseFields map[string]parsedField

func (pfs parseFields) elementToString(path string) (string, error) {
	refKey, ok := pfs[path]
	if !ok {
		return "", fmt.Errorf("could not find element at %q", path)
	}
	key, ok := refKey.value.(string)
	if !ok {
		return "", fmt.Errorf("element at %q is not a string: %v", path, refKey)
	}

	return key, nil
}

// configMapReferenceResolve resolves a ConfigMap reference returning
// name, key strings and an error.
func (pfs parseFields) configMapReferenceResolve(cms *corev1.ConfigMapKeySelector) (string, string, error) {
	name, err := pfs.elementToString(cms.Name)
	if err != nil {
		return "", "", fmt.Errorf("could not get reference to ConfigMap name: %v", err)
	}

	key, err := pfs.elementToString(cms.Key)
	if err != nil {
		return "", "", fmt.Errorf("could not get reference to ConfigMap key: %v", err)
	}

	return name, key, nil
}

func (pfs parseFields) configMapReferenceToEnvVarSource(cms *corev1.ConfigMapKeySelector) (*corev1.EnvVarSource, error) {
	n, k, err := pfs.configMapReferenceResolve(cms)
	if err != nil {
		return nil, err
	}

	return &corev1.EnvVarSource{
		ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: n,
			},
			Key: k,
		},
	}, nil
}

// secretReferenceResolve resolves a Secret reference returning
// name, key strings and an error.
func (pfs parseFields) secretReferenceResolve(ss *corev1.SecretKeySelector) (string, string, error) {
	name, err := pfs.elementToString(ss.Name)
	if err != nil {
		return "", "", fmt.Errorf("could not get reference to Secret name: %v", err)
	}

	key, err := pfs.elementToString(ss.Key)
	if err != nil {
		return "", "", fmt.Errorf("could not get reference to Secret key: %v", err)
	}

	return name, key, nil
}

func (pfs parseFields) secretReferenceToEnvVarSource(ss *corev1.SecretKeySelector) (*corev1.EnvVarSource, error) {
	n, k, err := pfs.secretReferenceResolve(ss)
	if err != nil {
		return nil, err
	}

	return &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: n,
			},
			Key: k,
		},
	}, nil
}

func (pfs parseFields) volumeReferenceToVolume(v *commonv1alpha1.FromSpecToVolume) (*commonv1alpha1.FromSpecToVolume, error) {
	fsv := &commonv1alpha1.FromSpecToVolume{
		Name:      v.Name,
		Path:      v.Path,
		MountPath: v.MountPath,
	}

	switch {
	case v.MountFrom.ConfigMap != nil:
		n, k, err := pfs.configMapReferenceResolve(v.MountFrom.ConfigMap)
		if err != nil {
			return nil, err
		}
		fsv.MountFrom.ConfigMap = &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: n,
			},
			Key: k,
		}

	case v.MountFrom.Secret != nil:
		n, k, err := pfs.secretReferenceResolve(v.MountFrom.Secret)
		if err != nil {
			return nil, err
		}
		fsv.MountFrom.Secret = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: n,
			},
			Key: k,
		}

	default:
		return nil, errors.New("volume reference needs to be a Secret or ConfigMap")
	}

	return fsv, nil

}

// restructure incoming object into a parsing friendly structure that
// can be referred to using 'friendly' JSON.
func (r *renderer) restructureIntoParsedFields(root map[string]interface{}, branch []string) map[string]parsedField {
	// Parsed fields indexed by JSONPath
	parsedFields := map[string]parsedField{}

	for k, v := range root {
		iter := append(branch, k)

		switch t := v.(type) {
		case map[string]interface{}:
			// Drill down intermediate nodes.

			// Send a copy of the slice and not a reference to it to
			// avoid the call from modifying the value.
			newBranch := make([]string, len(iter))
			copy(newBranch, iter)

			children := r.restructureIntoParsedFields(t, newBranch)
			for k, v := range children {
				parsedFields[k] = v
			}

			pf := parsedField{
				branch: iter,
				value:  v,
				// Element contains sub-nodes which will probably be rendered.
				// Mark it as intermediate node and let the renderer decide if this
				// should be skipped or not.
				intermediateNode: true,
			}
			parsedFields[pf.toJSONPath()] = pf

		case []interface{}:
			// When running into an arrays we need to check if the
			// items are primitives or maps. In the case of maps
			// we want to drill down further.
			arrayValues := make(map[string]parsedField, len(t))

			for i, v := range t {
				// for each element of the array we add the bracket
				// surrounded ordinal to the branch.
				iterArray := append(iter, fmt.Sprintf("[%d]", i))

				switch item := v.(type) {
				case map[string]interface{}:
					// This structure might look like this:
					// array:
					// - element1: value1
					//   element2: value2
					//
					// Drill down to continue inspecting those child items.
					children := r.restructureIntoParsedFields(item, iterArray)

					// For this type of array store both, the raw value
					// and the parsed fields of each item of the array.
					pf := parsedField{
						branch: iterArray,
						value:  item,
						array:  children,
					}
					arrayValues[pf.toJSONPath()] = pf

				default:
					// This array contains primitives:
					// array:
					// - foo
					// - bar
					//
					// keep each of them as regular leaf nodes.
					pf := parsedField{
						branch: iterArray,
						value:  item,
					}
					arrayValues[pf.toJSONPath()] = pf

				}
			}

			// Arrays keep both, the raw value for the whole array and
			// a drilled down array for each value. This enables later
			// processing to use the raw value to serialize (maybe JSON),
			// or use each independent value for processing.
			pf := parsedField{
				branch: iter,
				array:  arrayValues,
				value:  v,
			}
			parsedFields[pf.toJSONPath()] = pf

		default:
			// Leaf node, keep the value.
			pf := parsedField{
				branch: iter,
				value:  v,
			}
			parsedFields[pf.toJSONPath()] = pf
		}
	}

	return parsedFields
}

func (r *renderer) resolveAddress(ctx context.Context, namespace, path string, pfv interface{}) (string, error) {
	value, err := json.Marshal(pfv)
	if err != nil {
		return "", fmt.Errorf("could not parse reference structure as JSON at %q: %w", path, err)
	}

	// Convert json string to struct
	ref := &Reference{}
	if err := json.Unmarshal(value, &ref); err != nil {
		return "", fmt.Errorf("not valid reference structure at %q: %w", path, err)
	}

	if ref.Namespace == "" {
		ref.Namespace = namespace
	}

	uri, err := r.resolver.Resolve(ctx, &commonv1alpha1.Reference{
		APIVersion: ref.APIVersion,
		Kind:       ref.Kind,
		Namespace:  ref.Namespace,
		Name:       ref.Name,
	})
	if err != nil {
		return "", fmt.Errorf("could not resolve reference at %q: %w", path, err)
	}

	return uri, nil
}

func (r *renderer) renderStatus(obj reconciler.Object) error {
	errs := []string{}
	for i := range r.addStatus {
		sae := r.addStatus[i]

		path := strings.Split(sae.Path, ".")

		switch {
		case sae.Render.ValueFromParameter != nil:
			ev := obj.GetEnvVarAtPath(sae.Render.ValueFromParameter.Path)
			if ev == nil {
				continue
			}

			if err := obj.GetStatusManager().SetValue(ev.Value, path...); err != nil {
				// We lose stacktrace but process all status options.
				errs = append(errs, err.Error())
			}
		}
	}

	if len(errs) != 0 {
		msg := strings.Join(errs, ". ")
		return fmt.Errorf(msg[:len(msg)-2])
	}

	return nil
}

type Rendered interface {
	GetPodSpecOptions() []resources.PodSpecOption
	GetEnvVarByPath(path string) *corev1.EnvVar
	GetEnvVarByName(name string) *corev1.EnvVar
}

type Reference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
}

func normalizePath(path string) string {
	return strings.TrimLeft(path, "$.")
}

// Built-in function that resolves an address.
//
// Expected YAML element is:
//
// element:
//
//	  ref:
//	    apiVersion:
//	    group:
//	    kind:
//		   name:
//	 uri:
func (r *renderer) builtInResolveAddress(ctx context.Context, pf *parsedField, namespace, evName string) (*corev1.EnvVar, error) {
	addressable, ok := pf.value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected addressable structure: %+v", pf.value)
	}

	if uri, ok := addressable["uri"]; ok {
		value, ok := uri.(string)
		if !ok {
			return nil, errors.New("uri value is not a string")
		}

		return &corev1.EnvVar{
			Name:  evName,
			Value: value,
		}, nil

	}

	ref, ok := addressable["ref"]
	if !ok {
		return nil, fmt.Errorf("ref or uri must be informed: %+v", pf)
	}

	uri, err := r.resolveAddress(ctx, namespace, pf.toJSONPath(), ref)
	if err != nil {
		return nil, err
	}

	return &corev1.EnvVar{
		Name:  evName,
		Value: uri,
	}, nil
}

func (r *renderer) defaultRendering(pf *parsedField, evName string) (*corev1.EnvVar, error) {
	ev := &corev1.EnvVar{
		Name: evName,
	}

	switch {
	case pf.array != nil:
		// primitive indicates that all elements in an array are the
		// same primitive. We pre-create the primitive array to avoid
		// a second loop.
		primitive := true
		primitiveArr := []string{}

		// preserve order for arrays by iterating using the map key,
		// which contains the ornidal
		paths := make([]string, 0, len(pf.array))
		for path := range pf.array {
			paths = append(paths, path)
		}
		sort.Strings(paths)

		for _, p := range paths {
			switch v := pf.array[p].value.(type) {
			case map[string]interface{}:
				primitive = false
			default:
				primitiveArr = append(primitiveArr, fmt.Sprintf("%v", v))
			}
		}

		// If the array contains primitives, return a comma separated string
		if primitive {
			ev.Value = strings.Join(primitiveArr, ",")
		} else {
			// If the array contains complex structures, return a JSON serialization
			vb, err := json.Marshal(pf.value)
			if err != nil {
				return nil, err
			}
			ev.Value = string(vb)
		}

	case pf.value != nil:
		// Primitive values
		switch v := pf.value.(type) {
		case string:
			ev.Value = v

		default:
			vb, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			ev.Value = string(vb)
		}
	default:
		return nil, fmt.Errorf("unexpected incoming object structure at: %+v", *pf)
	}

	return ev, nil
}
