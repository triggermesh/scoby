// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"math/rand"
	"net/http"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	hookv1 "github.com/triggermesh/scoby/pkg/apis/hook/v1"
)

const (
	ConditionType     = "HookReportedStatus"
	ConditionReasonOK = "HOOKREPORTSOK"
)

func main() {
	h := &HandlerV1{
		// initialize random generator for the finalizer.
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/", h)
	mux.Handle("/v1", h)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, fmt.Sprintf("no resource at path %q", html.EscapeString(r.URL.String())), http.StatusNotFound)
	})

	srv := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("starting kuard hook")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}

// HandlerV1 is an example hooks server.
type HandlerV1 struct {
	rnd *rand.Rand
}

func (h *HandlerV1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hreq := &hookv1.HookRequest{}
	if err := json.NewDecoder(r.Body).Decode(hreq); err != nil {
		emsg := fmt.Errorf("cannot decode request into HookRequest: %v", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	// This hook example supports deployment and knative service form factor:
	switch hreq.FormFactor.Name {
	case "deployment":
		h.ServeDeploymentHook(w, hreq)

	case "ksvc":
		h.ServeKsvcHook(w, hreq)

	default:
		emsg := fmt.Errorf("request for formfactor %q not supported", hreq.FormFactor.Name)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
	}
}

func (h *HandlerV1) ServeDeploymentHook(w http.ResponseWriter, r *hookv1.HookRequest) {
	switch r.Phase {
	case hookv1.PhasePreReconcile:
		h.deploymentPreReconcile(w, r)

	case hookv1.PhaseFinalize:
		h.deploymentFinalize(w, r)

	default:
		emsg := fmt.Errorf("request for phase %q not supported", r.Phase)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
	}
}

func (h *HandlerV1) deploymentPreReconcile(w http.ResponseWriter, r *hookv1.HookRequest) {
	log.Printf("pre-reconcile deployment")

	// deployment form factor creates a "deployment" children entry
	ch, ok := r.Children["deployment"]
	if !ok {
		emsg := errors.New("children deployment element not found")
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	d := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(ch.Object, d); err != nil {
		emsg := fmt.Errorf("malformed deployment at children element: %w", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	// This hook is going to modify elements of the first container
	cs := d.Spec.Template.Spec.Containers
	if len(cs) == 0 {
		emsg := errors.New("children deployment has no containers")
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	cs[0].Env = append(cs[0].Env,
		corev1.EnvVar{
			Name:  "FROM_HOOK_VAR",
			Value: "this value is set from the hook",
		})

	cs[0].Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(1024*1024*100, resource.BinarySI),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: *resource.NewMilliQuantity(250, resource.DecimalSI),
		},
	}

	// Write the new object back to the children element and use it
	// at the hook's reply.

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
	if err != nil {
		emsg := fmt.Errorf("could not convert modified deployment into unstructured: %w", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	r.Children["deployment"] = &unstructured.Unstructured{Object: u}

	if err := h.setHookStatusPreReconcile(w, &r.Object, string(corev1.ConditionTrue), ConditionReasonOK); err != nil {
		emsg := fmt.Errorf("error setting object status: %w", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusInternalServerError)
		return
	}

	hres := &hookv1.HookResponse{
		Object:   &r.Object,
		Children: r.Children,
	}

	if err := json.NewEncoder(w).Encode(hres); err != nil {
		emsg := fmt.Errorf("error encoding response: %w", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
}

func (h *HandlerV1) deploymentFinalize(w http.ResponseWriter, r *hookv1.HookRequest) {
	log.Printf("finalize deployment")

	h.randomFinalize(w)
}

func (h *HandlerV1) ServeKsvcHook(w http.ResponseWriter, r *hookv1.HookRequest) {
	switch r.Phase {
	case hookv1.PhasePreReconcile:
		h.ksvcPreReconcile(w, r)
	case hookv1.PhaseFinalize:
		h.ksvcFinalize(w, r)
	}

	emsg := fmt.Errorf("request for phase %q not supported", r.Phase)
	logError(emsg)
	http.Error(w, emsg.Error(), http.StatusBadRequest)
}

func (h *HandlerV1) ksvcPreReconcile(w http.ResponseWriter, r *hookv1.HookRequest) {
	log.Printf("pre-reconcile ksvc")

	// knative service form factor creates a "ksvc" children entry
	ch, ok := r.Children["ksvc"]
	if !ok {
		emsg := errors.New("children ksvc element not found")
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	ksvc := servingv1.Service{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(ch.Object, ksvc); err != nil {
		emsg := fmt.Errorf("malformed ksvc at children element: %w", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	// This hook is going to modify elements of the first container
	cs := ksvc.Spec.Template.Spec.Containers
	if len(cs) == 0 {
		emsg := errors.New("children knative service has no containers")
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	cs[0].Env = append(cs[0].Env,
		corev1.EnvVar{
			Name:  "FROM_HOOK",
			Value: "value set from hook",
		})

	cs[0].Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(1024*1024*100, resource.BinarySI),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: *resource.NewMilliQuantity(250, resource.DecimalSI),
		},
	}

	// Write the new object back to the children element and use it
	// at the hook's reply.

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ksvc)
	if err != nil {
		emsg := fmt.Errorf("could not convert modified knative service into unstructured: %w", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusBadRequest)
		return
	}

	r.Children["ksvc"] = &unstructured.Unstructured{Object: u}

	if err := h.setHookStatusPreReconcile(w, &r.Object, string(corev1.ConditionTrue), ConditionReasonOK); err != nil {
		emsg := fmt.Errorf("error setting object status: %w", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusInternalServerError)
		return
	}

	hres := &hookv1.HookResponse{
		Object:   &r.Object,
		Children: r.Children,
	}

	if err := json.NewEncoder(w).Encode(hres); err != nil {
		emsg := fmt.Errorf("error encoding response: %w", err)
		logError(emsg)
		http.Error(w, emsg.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
}

func (h *HandlerV1) ksvcFinalize(w http.ResponseWriter, r *hookv1.HookRequest) {
	log.Printf("finalize ksvc")

	h.randomFinalize(w)
}

func (h *HandlerV1) setHookStatusPreReconcile(w http.ResponseWriter, u *unstructured.Unstructured, status, reason string) error {
	if conditions, ok, _ := unstructured.NestedSlice(u.Object, "status", "conditions"); ok {
		var hookCondition map[string]interface{}

		// Look for existing condition
		for i := range conditions {
			c, ok := conditions[i].(map[string]interface{})
			if !ok {
				return fmt.Errorf("wrong condition format: %+v", conditions[i])
			}

			t, ok := c["type"].(string)
			if !ok {
				return fmt.Errorf("wrong condition type: %+v", c["type"])
			}

			if ok && t == ConditionType {
				hookCondition = conditions[i].(map[string]interface{})
				break
			}
		}

		// If the condition does not exist, create it.
		if hookCondition == nil {
			hookCondition = map[string]interface{}{
				"type": ConditionType,
			}
			conditions = append(conditions, hookCondition)
		}

		hookCondition["status"] = status
		hookCondition["reason"] = reason

		if err := unstructured.SetNestedSlice(u.Object, conditions, "status", "conditions"); err != nil {
			return err
		}
	}

	annotations, ok, _ := unstructured.NestedStringMap(u.Object, "status", "annotations")
	if !ok {
		annotations = map[string]string{}
	}

	annotations["greetings"] = "from hook"

	return unstructured.SetNestedStringMap(u.Object, annotations, "status", "annotations")
}

func (h *HandlerV1) randomFinalize(w http.ResponseWriter) {
	// If 0 cancel  finalization, if 1 let it happen
	if h.rnd.Intn(2) == 0 {
		log.Printf("canceling finalization ...")

		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")

		_false := false
		herr := &hookv1.HookResponseError{
			Message:   "finalization denied from hook",
			Permanent: &_false,
		}

		if err := json.NewEncoder(w).Encode(herr); err != nil {
			emsg := fmt.Errorf("error encoding response: %w", err)
			logError(emsg)
			http.Error(w, emsg.Error(), http.StatusInternalServerError)
		}
	} else {
		log.Printf("hook says yes to finalization")
	}
}

func logError(err error) {
	log.Printf("Error: %v\n", err)
}
