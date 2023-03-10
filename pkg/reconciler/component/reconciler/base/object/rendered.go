package object

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

// rootObject where the renderer will start inspecting
const rootObject = "spec"

type Renderer interface {
	Render(obj Reconciling) (Rendered, error)
}

type renderer struct {
	containerName  string
	containerImage string

	// JSONPath indexed configuration parameters.
	// configuration map[string]apicommon.CustomizeParameterConfiguration
	customization map[string]apicommon.CustomizeParameterConfiguration

	// Global options to be applied while transforming object fields
	// into workload parameters.
	global apicommon.GlobalParameterConfiguration

	// Static set of environment variables to be added to as
	// parameters to the workload.
	addEnv []corev1.EnvVar
}

func NewRenderer(containerName, containerImage string, configuration apicommon.ParameterConfiguration) Renderer {
	r := &renderer{
		containerName:  containerName,
		containerImage: containerImage,
	}

	if configuration.Global != nil {
		r.global = *configuration.Global
	}

	// Curate object fields customization, index them by their
	// relaxed JSONPath.
	if configuration.Customize != nil {
		r.customization = make(map[string]apicommon.CustomizeParameterConfiguration, len(configuration.Customize))
		for _, c := range configuration.Customize {
			r.customization[strings.TrimLeft(c.Path, "$.")] = c
		}
	}

	// Keep the list of extra environment variables to be appended .
	if configuration.AddEnvs != nil && len(configuration.AddEnvs) != 0 {
		r.addEnv = make([]corev1.EnvVar, len(configuration.AddEnvs))
		copy(configuration.AddEnvs, r.addEnv)
	}

	return r
}

func (r *renderer) Render(obj Reconciling) (Rendered, error) {
	uobj, ok := obj.AsKubeObject().(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("could not parse object into unstructured: %s", obj.GetName())
	}

	rendered := &renderedObject{}

	// not having a spec is possible, return empty renderedObject
	uobjRoot, ok := uobj.Object[rootObject]
	if !ok {
		return rendered, nil
	}

	root, ok := uobjRoot.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("object %q is expected to be a map[string]interface{}", rootObject)
	}

	// do a first pass of the unstructured and turn it into an
	// structure that can be used to apply the registered configuration.
	parsedFields := restructureIntoParsedFields(root, []string{rootObject})

	// TODO status info

	ro, err := r.renderParsedFields(parsedFields)
	if err != nil {
		return nil, err
	}

	// Add reference to the rendered object
	ro.obj = obj

	return ro, nil
}

func (r *renderer) renderParsedFields(pfs map[string]parsedField) (*renderedObject, error) {

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
	// indexed by JSONPath.
	rendered := &renderedObject{
		evs: map[string]corev1.EnvVar{},
	}

	// Keep each environment variable key to be able to sort.
	envNames := []string{}

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
		ev := corev1.EnvVar{
			Name: strings.ToUpper(strings.Join(pf.branch[1:], "_")),
		}

		// Check soon if the value needs to be skipped, move over to the next.
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
				return nil, fmt.Errorf("could not find reference to ConfigMap at %q", renderConfig.ValueFromConfigMap.Name)
			}
			name, ok := refName.value.(string)
			if !ok {
				return nil, fmt.Errorf("reference to ConfigMap at %q is not a string: %v", renderConfig.ValueFromConfigMap.Name, refName)
			}
			refKey, ok := pfs[renderConfig.ValueFromConfigMap.Key]
			if !ok {
				return nil, fmt.Errorf("could not find reference to ConfigMap key at %q", renderConfig.ValueFromConfigMap.Name)
			}
			key, ok := refKey.value.(string)
			if !ok {
				return nil, fmt.Errorf("reference to ConfigMap key at %q is not a string: %v", renderConfig.ValueFromConfigMap.Name, refKey)
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
				return nil, fmt.Errorf("could not find reference to Secret at %q", renderConfig.ValueFromSecret.Name)
			}
			name, ok := refName.value.(string)
			if !ok {
				return nil, fmt.Errorf("reference to Secret at %q is not a string: %v", renderConfig.ValueFromSecret.Name, refName)
			}
			refKey, ok := pfs[renderConfig.ValueFromSecret.Key]
			if !ok {
				return nil, fmt.Errorf("could not find reference to Secret key at %q", renderConfig.ValueFromSecret.Name)
			}
			key, ok := refKey.value.(string)
			if !ok {
				return nil, fmt.Errorf("reference to Secret key at %q is not a string: %v", renderConfig.ValueFromSecret.Name, refKey)
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
			// TODO

			// If there are further internal elements, avoid
			// parsing them.
			avoidFieldPrefixes = append(avoidFieldPrefixes, k)
		}

		envNames = append(envNames, ev.Name)
		rendered.evs[ev.Name] = ev
	}

	// Prepare the result as an ordered set of options.
	copts := []resources.ContainerOption{
		resources.ContainerWithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError),
	}

	sort.Strings(envNames)
	for _, k := range envNames {
		ev := rendered.evs[k]
		copts = append(copts, resources.ContainerAddEnv(&ev))
	}

	rendered.podSpecOptions = []resources.PodSpecOption{
		resources.PodSpecAddContainer(
			resources.NewContainer(r.containerName, r.containerImage, copts...),
		),
	}

	return rendered, nil
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
func restructureIntoParsedFields(root map[string]interface{}, branch []string) map[string]parsedField {
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

			children := restructureIntoParsedFields(t, newBranch)
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
					children := restructureIntoParsedFields(item, iterArray)

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

type Rendered interface {
	GetPodSpecOptions() []resources.PodSpecOption
	GetAddressURL() string
}

type renderedObject struct {
	// Reference to the reconciled object that generates
	// this rendering.
	obj Reconciling

	// Environment variables to be added to the workload,
	// mapped by their JSON path.
	//
	// These values are stored to be able to use them
	// in for calculations.
	evs map[string]corev1.EnvVar

	// pre-baked PodSpecOptions including the workload container
	podSpecOptions []resources.PodSpecOption

	// address where the workload service (if any) can be reached.
	addressURL string
}

// GetPodSpecOptions for the workload, including the configured container.
func (r *renderedObject) GetPodSpecOptions() []resources.PodSpecOption {
	return r.podSpecOptions
}

func (r *renderedObject) GetAddressURL() string {
	return r.addressURL
}
