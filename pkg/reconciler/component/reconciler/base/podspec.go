// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	// "k8s.io/client-go/util/jsonpath"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

const rootObject = "spec"

type PodSpecRenderer interface {
	Render(obj ReconciledObject) ([]resources.PodSpecOption, error)
}

type podSpecRenderer struct {
	name  string
	image string

	// JSONPath indexed configuration parameters.
	configuration map[string]apicommon.Parameter
}

func NewPodSpecRenderer(name, image string, configuration *apicommon.ParameterConfiguration) PodSpecRenderer {
	// index the path into a map to save time looking for rules.
	configParameters := map[string]apicommon.Parameter{}
	for i := range configuration.Parameters {
		// sanitize JSONPath to make it match the expected path at the parser
		path := strings.TrimLeft(configuration.Parameters[i].Path, "$.")
		configParameters[path] = configuration.Parameters[i]
	}

	return &podSpecRenderer{
		name:          name,
		image:         image,
		configuration: configParameters,
	}
}

func (r *podSpecRenderer) Render(obj ReconciledObject) ([]resources.PodSpecOption, error) {
	pso := []resources.PodSpecOption{}
	opts, err := r.parseObjectIntoContainer(obj)
	if err != nil {
		return pso, err
	}

	opts = append(opts, resources.ContainerWithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError))

	return []resources.PodSpecOption{resources.PodSpecAddContainer(
		resources.NewContainer(r.name, r.image, opts...),
	)}, nil
}

type parsedValue struct {
	branch   []string
	value    interface{}
	array    []parsedValue
	jsonPath string
}

func (v *parsedValue) toJSONPath() string {
	return strings.Join(v.branch, ".")
}

func (r *podSpecRenderer) parseObjectIntoContainer(obj ReconciledObject) ([]resources.ContainerOption, error) {
	copts := []resources.ContainerOption{}

	uobj, ok := obj.AsKubeObject().(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("could not parse object into unstructured: %s", obj.GetName())
	}

	objRoot, ok := uobj.Object[rootObject]
	if !ok {
		return copts, nil
	}

	root, ok := objRoot.(map[string]interface{})
	if !ok {
		return copts, errors.New("object spec is expected to be a map[string]interface{}")
	}

	parsedFields := parseFields(root, []string{rootObject}, rootObject)

	fmt.Printf("\nDEBUG DELETEME summarizing parsed fields ...\n\n")
	for _, p := range parsedFields {
		if p.array != nil {
			fmt.Printf("DEBUG DELETEME ARRAY FOR %s/%+v.\n", p.jsonPath, p.branch)
			for _, pp := range p.array {
				if pp.array != nil {
					fmt.Printf("DEBUG DELETEME %s/%+v. ARR:%+v\n", pp.jsonPath, p.branch, pp.array)
				} else {
					fmt.Printf("DEBUG DELETEME %s/%+v. VAL:%+v\n", pp.jsonPath, p.branch, pp.value)
				}
			}

		} else {
			fmt.Printf("DEBUG DELETEME %s. VAL: %v\n", p.jsonPath, p.value)
		}
	}

	mynew := true

	if mynew {
		valuesCopts, err := r.valuesToContainerOptions(parsedFields)
		if err != nil {
			return nil, err
		}

		return append(copts, valuesCopts...), nil
	}

	keys := []string{}
	kv := map[string]string{}
	for i := range parsedFields {

		// key is rendered as the element path (omiting the root element)
		// joined by underscores and uppercased
		key := strings.ToUpper(strings.Join(parsedFields[i].branch[1:], "_"))
		value := ""

		switch v := parsedFields[i].value.(type) {
		case string:
			value = v
		case []interface{}:

			// Empty values produce empty env vars
			lv := len(v)
			if lv == 0 {
				break
			}

			// analyze the first element, but still it could happen that the slice is
			// irregular. Make sure we don't panic in such case.
			switch v[0].(type) {
			case []interface{}:
				// TODO if there is a "[*]" jsonpath expression process it.

				//
			case string, int, float64, bool:
				lv--
				for i, s := range v {
					value += fmt.Sprintf("%v", s)
					if i < lv {
						value += ","
					}
				}

				// fmt.Printf("DEBUG DELETEME2 field %+v value type is %T value is %s\n", parsedFields[i], v, value)

			default:
				// fmt.Printf("DEBUG DELETEME2 do nothing for field %+v value type is %T value is %s\n", parsedFields[i], v, value)
			}

			// if _, ok := v[0].(string); ok {
			// 	lv--
			// 	for i, s := range v {
			// 		// make sure we don't panic in case the slice is irregular
			// 		if vs, ok := s.(string); ok {
			// 			value += vs
			// 		} else {
			// 			value += fmt.Sprintf("%v", s)
			// 		}

			// 		if i < lv {
			// 			value += ","
			// 		}
			// 	}
			// 	break
			// }

			// if _, ok := v[0].(int), ; ok {
			// 	lv--
			// 	for i, s := range v {
			// 		value += fmt.Sprintf("%d", s.(int))
			// 		if i < lv {
			// 			value += ","
			// 		}
			// 	}
			// 	break
			// }

			// if _, ok := v[0].(float64); ok {
			// 	lv--
			// 	for i, s := range v {
			// 		value += fmt.Sprintf("%f", s.(float64))
			// 		if i < lv {
			// 			value += ","
			// 		}
			// 	}
			// 	break
			// }

		default:

			vb, err := json.Marshal(parsedFields[i].value)
			if err != nil {
				return copts, err
			}
			value = string(vb)
		}

		keys = append(keys, key)
		kv[key] = value
	}

	sort.Strings(keys)
	for _, k := range keys {
		copts = append(copts, resources.ContainerAddEnvFromValue(k, kv[k]))
	}

	return copts, nil
}

// parse object into structured fields
func parseFields(root map[string]interface{}, branch []string, jsonPath string) []parsedValue {
	parsed := []parsedValue{}

	for k, v := range root {
		jsonPathK := jsonPath + "." + k
		iter := append(branch, k)

		switch t := v.(type) {
		case map[string]interface{}:
			values := parseFields(t, iter, jsonPathK)
			parsed = append(parsed, values...)

		case []interface{}:
			arrayValues := make([]parsedValue, 0, len(t))

			for i, v := range t {
				iterArray := append(iter, fmt.Sprintf("[%d]", i))
				switch item := v.(type) {
				case map[string]interface{}:

					values := parseFields(item, iterArray, fmt.Sprintf("%s[%d]", jsonPathK, i))

					arrayValues = append(arrayValues, parsedValue{
						branch:   iterArray,
						value:    item,
						array:    values,
						jsonPath: fmt.Sprintf("%s[%d]", jsonPathK, i),
					})

				default:
					arrayValues = append(arrayValues, parsedValue{
						branch:   iterArray,
						value:    item,
						jsonPath: fmt.Sprintf("%s[%d]", jsonPathK, i),
					})

				}
			}

			// inform both, the array values containing each single isolated value,
			// and the value containing the raw entry. This enable later processing of
			// each independent value or all values.
			parsed = append(parsed, parsedValue{
				branch:   iter,
				array:    arrayValues,
				value:    v,
				jsonPath: jsonPathK,
			})

		default:
			// fmt.Printf("DEBUG DELETEME.parseFields value found jsonpath %q: %+v\n", jsonPathK, v)
			parsed = append(parsed, parsedValue{
				branch:   iter,
				value:    v,
				jsonPath: jsonPathK,
			})

		}
	}

	return parsed
}

// converts an array of values to container options
func (r *podSpecRenderer) valuesToContainerOptions(parsedValues []parsedValue) ([]resources.ContainerOption, error) {
	fmt.Printf("\nDEBUG DELETEME: Entering valuesToContainerOptions\n\n")

	copts := []resources.ContainerOption{}
	keys := []string{}
	kv := map[string]string{}

	for _, parsedValue := range parsedValues {

		// variables for the parameter's key and value
		pKey := ""
		pValue := ""

		// Check rules for the this value's JSONPath.
		rule, ok := r.configuration[parsedValue.toJSONPath()]
		if ok {
			fmt.Printf("DEBUG DELETEME: rule found for jsonpath %q: %+v\n", parsedValue.toJSONPath(), rule)
			if rule.Skip != nil && *rule.Skip {
				continue
			}
			if rule.Render != nil {
				if rule.Render.Literal != nil {
					pKey = *rule.Render.Literal
				}
			}
		}

		// Keep the parameter key when set by a previous rule.
		if pKey == "" {
			pKey = strings.ToUpper(strings.Join(parsedValue.branch[1:], "_"))
		}

		switch {
		case parsedValue.array != nil:

			// TODO check if any rules need to be applied.

			// primitive indicates that all elements in an array are the
			// same primitive. We pre-create the primitive array to avoid
			// a second loop.
			primitive := true
			primitiveArr := []string{}

			for i := range parsedValue.array {
				switch v := parsedValue.array[i].value.(type) {
				case map[string]interface{}:
					primitive = false
				default:
					fmt.Printf("DEBUG DELETEME: primitive maybe  %q: %T\n", parsedValue.toJSONPath(), v)
					primitiveArr = append(primitiveArr, fmt.Sprintf("%v", v))
				}
			}

			// If the array contains primitives, return a comma separated string
			if primitive {
				pValue = strings.Join(primitiveArr, ",")
			} else {
				fmt.Printf("DEBUG DELETEME: not primitive  %q: %+v\n", parsedValue.toJSONPath(), parsedValue.value)
				// If the array contains complex structures, return a JSON serialization
				vb, err := json.Marshal(parsedValue.value)
				if err != nil {
					return nil, err
				}
				pValue = string(vb)
			}

		case parsedValue.value != nil:
			// primitive values
			switch v := parsedValue.value.(type) {
			case string:
				pValue = v
			default:

				vb, err := json.Marshal(v)
				if err != nil {
					return nil, err
				}
				pValue = string(vb)
			}
		}

		keys = append(keys, pKey)
		kv[pKey] = pValue
	}

	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("DEBUG DELETEME: %s/%s\n", k, kv[k])
		copts = append(copts, resources.ContainerAddEnvFromValue(k, kv[k]))
	}

	return copts, nil
}

// func (r *podSpecRenderer) valuesToKV(parsedValues []parsedValue) (map[string]string, error) {
// 	keys := []string{}
// 	kv := map[string]string{}

// 	for _, parsedValue := range parsedValues {

// 		// variables for the parameter's key and value
// 		pKey := ""
// 		pValue := ""

// 		// Check rules for the this value's JSONPath.
// 		rule, ok := r.configuration[parsedValue.toJSONPath()]
// 		if ok {
// 			fmt.Printf("DEBUG DELETEME: rule found for jsonpath %q: %+v\n", parsedValue.toJSONPath(), rule)
// 			if rule.Skip != nil && *rule.Skip {
// 				continue
// 			}
// 			if rule.Render != nil {
// 				if rule.Render.Literal != nil {
// 					pKey = *rule.Render.Literal
// 				}
// 			}
// 		}

// 		// Keep the parameter key when set by a previous rule.
// 		if pKey == "" {
// 			pKey = strings.ToUpper(strings.Join(parsedValue.branch[1:], "_"))
// 		}

// 		switch v := parsedValue.value.(type) {
// 		case string:
// 			pValue = v
// 		case []interface{}:

// 		default:

// 			vb, err := json.Marshal(v)
// 			if err != nil {
// 				return nil, err
// 			}
// 			pValue = string(vb)

// 		}

// 		keys = append(keys, pKey)
// 		kv[pKey] = pValue

// 		// if parsedValue.array != nil {
// 		// 	fmt.Printf("DEBUG DELETEME: array at jsonpath %q\n", parsedValue.toJSONPath())
// 		// } else {
// 		// 	fmt.Printf("DEBUG DELETEME: value at jsonpath %q\n", parsedValue.toJSONPath())
// 		// }
// 	}
// }
