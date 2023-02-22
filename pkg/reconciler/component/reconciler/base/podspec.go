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
	psr := &podSpecRenderer{
		name:          name,
		image:         image,
		configuration: map[string]apicommon.Parameter{},
	}

	// index the path into a map to save time looking for rules.
	if configuration != nil {
		for i := range configuration.Parameters {
			// sanitize JSONPath to make it match the expected path at the parser
			path := strings.TrimLeft(configuration.Parameters[i].Path, "$.")
			psr.configuration[path] = configuration.Parameters[i]
		}
	}

	return psr
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
	branch []string
	value  interface{}
	array  []parsedValue
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

	parsedFields := parseFields(root, []string{rootObject})

	valuesCopts, err := r.valuesToContainerOptions(parsedFields)
	if err != nil {
		return nil, err
	}

	return append(copts, valuesCopts...), nil
}

// parse object into structured fields to make them easy to process
// into workload parameters.
func parseFields(root map[string]interface{}, branch []string) []parsedValue {
	parsed := []parsedValue{}

	for k, v := range root {
		iter := append(branch, k)

		switch t := v.(type) {
		case map[string]interface{}:
			values := parseFields(t, iter)
			parsed = append(parsed, values...)

		case []interface{}:
			arrayValues := make([]parsedValue, 0, len(t))

			for i, v := range t {
				iterArray := append(iter, fmt.Sprintf("[%d]", i))
				switch item := v.(type) {
				case map[string]interface{}:

					values := parseFields(item, iterArray)

					arrayValues = append(arrayValues, parsedValue{
						branch: iterArray,
						value:  item,
						array:  values,
					})

				default:
					arrayValues = append(arrayValues, parsedValue{
						branch: iterArray,
						value:  item,
					})

				}
			}

			// inform both, the array values containing each single isolated value,
			// and the value containing the raw entry. This enable later processing of
			// each independent value or all values.
			parsed = append(parsed, parsedValue{
				branch: iter,
				array:  arrayValues,
				value:  v,
			})

		default:
			parsed = append(parsed, parsedValue{
				branch: iter,
				value:  v,
			})

		}
	}

	return parsed
}

// converts an array of values to container options
func (r *podSpecRenderer) valuesToContainerOptions(parsedValues []parsedValue) ([]resources.ContainerOption, error) {

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
			if rule.Skip != nil && *rule.Skip {
				continue
			}
			if rule.Render != nil {
				if rule.Render.Key != nil {
					pKey = *rule.Render.Key
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
					primitiveArr = append(primitiveArr, fmt.Sprintf("%v", v))
				}
			}

			// If the array contains primitives, return a comma separated string
			if primitive {
				pValue = strings.Join(primitiveArr, ",")
			} else {
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
		copts = append(copts, resources.ContainerAddEnvFromValue(k, kv[k]))
	}

	return copts, nil
}
