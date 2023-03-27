// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

// rootObject where the renderer will start inspecting
const (
	rootObject    = "spec"
	addEnvsPrefix = "$added."
)

func NewRenderer(wkl *apicommon.Workload, resolver reconciler.Resolver) reconciler.ObjectRenderer {
	r := &renderer{
		resolver: resolver,
	}

	if wkl.ParameterConfiguration != nil {
		pcfg := wkl.ParameterConfiguration

		if pcfg.Global != nil {
			r.global = *pcfg.Global
		}

		// Curate object fields customization, index them by their
		// relaxed JSONPath.
		if pcfg.Customize != nil && len(pcfg.Customize) != 0 {
			r.customization = make(map[string]apicommon.CustomizeParameterConfiguration, len(pcfg.Customize))
			for _, c := range pcfg.Customize {
				r.customization[strings.TrimLeft(c.Path, "$.")] = c
			}
		}
		// Keep the list of extra environment variables to be appended.
		if pcfg.AddEnvs != nil && len(pcfg.AddEnvs) != 0 {
			r.addEnvs = make([]corev1.EnvVar, len(pcfg.AddEnvs))
			copy(r.addEnvs, pcfg.AddEnvs)
		}
	}

	if wkl.StatusConfiguration != nil {
		scfg := wkl.StatusConfiguration

		if scfg.AddElements != nil && len(scfg.AddElements) != 0 {
			r.addStatus = make([]apicommon.StatusAddElement, len(scfg.AddElements))
			copy(r.addStatus, scfg.AddElements)
		}
	}

	return r
}

type renderer struct {
	resolver reconciler.Resolver

	// JSONPath indexed configuration parameters.
	// configuration map[string]apicommon.CustomizeParameterConfiguration
	customization map[string]apicommon.CustomizeParameterConfiguration

	// Global options to be applied while transforming object fields
	// into workload parameters.
	global apicommon.GlobalParameterConfiguration

	// Static set of environment variables to be added to as
	// parameters to the workload.
	addEnvs []corev1.EnvVar

	// Set of rules that add or fill elements at the object status.
	addStatus []apicommon.StatusAddElement
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

	// TODO Render Status
	err = r.renderStatus(obj)
	if err != nil {
		return err
	}

	return nil
}

func (r *renderer) renderParsedFields(ctx context.Context, obj reconciler.Object, pfs map[string]parsedField) error {

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

	// Generated environment variables are stored in the renderedObject.ev map,
	// indexed by JSONPath and environment variable name.
	// This structure will be kept at the rendered object to be used when a
	// calculation cross references a value from other element
	// rendered := &renderedObject{
	// 	evsByPath: map[string]*corev1.EnvVar{},
	// 	evsByName: map[string]*corev1.EnvVar{},
	// }

	// Keep each environment variable key to be able to sort.
	// envNames := []string{}

	// Add all added environment variables that are not related to
	// the object's data
	for _, ev := range r.addEnvs {
		// There is no path for added envrionment variables, but
		// we want to keep consistency, so we also add them here
		// using a prefix plus the variable name.
		obj.AddEnvVar(addEnvsPrefix+ev.Name, &ev)
	}

	// Iterate all elements in the parsed fields structure.
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
		// Retrieve custom render configuration for the field.
		var renderConfig *apicommon.ParameterRenderConfiguration
		if customize, ok := r.customization[pf.toJSONPath()]; ok && customize.Render != nil {
			renderConfig = customize.Render
		}

		// Create environment variable for this field.
		ev := &corev1.EnvVar{
			Name: strings.ToUpper(strings.Join(pf.branch[1:], "_")),
		}

		// Check soon if the value needs to be skipped, move over to the next.
		//
		// Note: maybe in the future we will find skip conbined with a function
		// that would need to generate a result, probably because some other
		// registration configuration references it.
		if renderConfig.IsSkip() {
			continue
		}

		// Skip intermediate nodes that have no customizations, they exist only
		// to allow them to be used for caluculations.
		// When customizations are defined they will need to parse to produce
		// an environment variable
		if pf.intermediateNode && renderConfig == nil {
			continue
		}

		// If key is overriden by customization set it, if not
		// apply global prefix.
		if key := renderConfig.GetKey(); key != "" {
			ev.Name = key
		} else if prefix := r.global.GetDefaultPrefix(); prefix != "" {
			ev.Name = prefix + ev.Name
		}

		switch {
		case !renderConfig.IsValueOverriden():
			// By default process the value depending on the type.
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
						return err
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
						return err
					}
					ev.Value = string(vb)

				}
			default:
				// TODO this is not expected
			}

		case renderConfig.Value != nil:
			ev.Value = *renderConfig.Value

			// If there are further internal elements, avoid
			// parsing them.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)

		case renderConfig.ValueFromConfigMap != nil:
			refName, ok := pfs[renderConfig.ValueFromConfigMap.Name]
			if !ok {
				return fmt.Errorf("could not find reference to ConfigMap at %q", renderConfig.ValueFromConfigMap.Name)
			}
			name, ok := refName.value.(string)
			if !ok {
				return fmt.Errorf("reference to ConfigMap at %q is not a string: %v", renderConfig.ValueFromConfigMap.Name, refName)
			}
			refKey, ok := pfs[renderConfig.ValueFromConfigMap.Key]
			if !ok {
				return fmt.Errorf("could not find reference to ConfigMap key at %q", renderConfig.ValueFromConfigMap.Name)
			}
			key, ok := refKey.value.(string)
			if !ok {
				return fmt.Errorf("reference to ConfigMap key at %q is not a string: %v", renderConfig.ValueFromConfigMap.Name, refKey)
			}

			ev.ValueFrom = &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					},
					Key: key,
				},
			}

			// If there are further internal elements, avoid
			// parsing them.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)

		case renderConfig.ValueFromSecret != nil:
			refName, ok := pfs[renderConfig.ValueFromSecret.Name]
			if !ok {
				return fmt.Errorf("could not find reference to Secret at %q", renderConfig.ValueFromSecret.Name)
			}
			name, ok := refName.value.(string)
			if !ok {
				return fmt.Errorf("reference to Secret at %q is not a string: %v", renderConfig.ValueFromSecret.Name, refName)
			}
			refKey, ok := pfs[renderConfig.ValueFromSecret.Key]
			if !ok {
				return fmt.Errorf("could not find reference to Secret key at %q", renderConfig.ValueFromSecret.Name)
			}
			key, ok := refKey.value.(string)
			if !ok {
				return fmt.Errorf("reference to Secret key at %q is not a string: %v", renderConfig.ValueFromSecret.Name, refKey)
			}

			ev.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					},
					Key: key,
				},
			}

			// If there are further internal elements, avoid
			// parsing them.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)

		case renderConfig.ValueFromBuiltInFunc != nil:
			switch renderConfig.ValueFromBuiltInFunc.Name {
			case "resolveAddress":
				// element:
				//   ref:
				//     apiVersion:
				//     group:
				//     kind:
				// 	   name:
				//  uri:

				addressable, ok := pf.value.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unexpected addressable structure at  %q: %+v", k, pf.value)
				}

				if uri, ok := addressable["uri"]; ok {
					value, ok := uri.(string)
					if !ok {
						return fmt.Errorf("uri value at %q is not a string", k)
					}
					ev.Value = value
				} else if ref, ok := addressable["ref"]; ok {
					uri, err := r.resolveAddress(ctx, obj.GetNamespace(), k, ref)
					if err != nil {
						return err
					}
					ev.Value = uri
				}
			}

			// If there are further internal elements, avoid
			// parsing them.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)
		}

		// envNames = append(envNames, ev.Name)
		obj.AddEnvVar(k, ev)
		// rendered.evsByName[ev.Name] = ev
		// rendered.evsByPath[k] = ev
	}

	// // Prepare the result as an ordered set of options.
	// copts := []resources.ContainerOption{
	// 	resources.ContainerWithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError),
	// }

	// sort.Strings(envNames)
	// for _, k := range envNames {
	// 	ev := rendered.evsByName[k]
	// 	copts = append(copts, resources.ContainerAddEnv(ev))
	// }

	// rendered.podSpecOptions = []resources.PodSpecOption{
	// 	resources.PodSpecAddContainer(
	// 		resources.NewContainer(r.containerName, r.containerImage, copts...),
	// 	),
	// }

	// return rendered, nil
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

	uri, err := r.resolver.Resolve(ctx, &corev1.ObjectReference{
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

// type renderedObject struct {
// 	// Reference to the reconciled object that generates
// 	// this rendering.
// 	obj Reconciling

// 	// Environment variables to be added to the workload,
// 	// mapped by their JSON path and Name.
// 	//
// 	// These values are stored to be able to use them
// 	// for calculations.
// 	evsByPath map[string]*corev1.EnvVar
// 	evsByName map[string]*corev1.EnvVar

// 	// pre-baked PodSpecOptions including the workload container
// 	podSpecOptions []resources.PodSpecOption
// }

// // GetPodSpecOptions for the workload, including the configured container.
// func (r *renderedObject) GetPodSpecOptions() []resources.PodSpecOption {
// 	return r.podSpecOptions
// }

// // GetEnvVarByPath given an object data path, returns the associated
// // environment variable. Nil when not found.
// func (r *renderedObject) GetEnvVarByPath(path string) *corev1.EnvVar {
// 	return r.evsByPath[path]
// }

// // GetEnvVarByName given an object data path, returns the associated
// // environment variable. Nil when not found.
// func (r *renderedObject) GetEnvVarByName(name string) *corev1.EnvVar {
// 	return r.evsByName[name]
// }

type Reference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
}
