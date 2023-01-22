// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

const rootObject = "spec"

type PodSpecRenderer interface {
	Render(obj client.Object) ([]resources.PodSpecOption, error)
}

type renderer struct {
	name  string
	image string
}

func NewPodSpecRenderer(name, image string) PodSpecRenderer {
	return &renderer{
		name:  name,
		image: image,
	}
}

func (r *renderer) Render(obj client.Object) ([]resources.PodSpecOption, error) {
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

type value struct {
	branch []string
	value  interface{}
}

func (r *renderer) parseObjectIntoContainer(obj client.Object) ([]resources.ContainerOption, error) {
	copts := []resources.ContainerOption{}

	uobj, ok := obj.(*unstructured.Unstructured)
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

	parsedFields := parseFields(root, []string{})

	keys := []string{}
	kv := map[string]string{}
	for i := range parsedFields {

		// key is rendered as the element path (omiting the root element)
		// joined by underscores and uppercased
		key := strings.ToUpper(strings.Join(parsedFields[i].branch, "_"))
		value := ""

		switch parsedFields[i].value.(type) {
		case string:
			value = parsedFields[i].value.(string)
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

func parseFields(root map[string]interface{}, branch []string) []value {

	parsed := []value{}

	for k, v := range root {
		iter := append(branch, k)
		switch t := v.(type) {
		case map[string]interface{}:
			values := parseFields(t, iter)
			parsed = append(parsed, values...)

		case string:
			parsed = append(parsed, value{
				branch: iter,
				value:  v,
			})

		case int:
			parsed = append(parsed, value{
				branch: iter,
				value:  v,
			})

		case bool:
			parsed = append(parsed, value{
				branch: iter,
				value:  v,
			})

		default:
			parsed = append(parsed, value{
				branch: iter,
				value:  v,
			})

		}
	}

	return parsed
}
