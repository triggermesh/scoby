package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-logr/logr"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	hookv1 "github.com/triggermesh/scoby/pkg/hook/v1"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
)

const (
	addEnvsPrefix = "$hook."
)

type hookReconciler struct {
	url         string
	isFinalizer bool

	log logr.Logger
}

func New(h *commonv1alpha1.Hook, url string, log logr.Logger) reconciler.HookReconciler {

	hr := &hookReconciler{
		url: url,
		log: log,
		// by default finalization is considered implemented.
		isFinalizer: true,
	}

	if h.Finalization != nil && !*h.Finalization.Enabled {
		hr.isFinalizer = false
	}

	return hr
}

func (hr *hookReconciler) Reconcile(ctx context.Context, obj reconciler.Object) error {
	hr.log.V(1).Info("Reconciling at hook", "obj", obj)

	res, err := hr.requestHook(ctx, hookv1.OperationReconcile, obj)
	if err != nil {
		return err
	}

	// TODO use status and env vars
	hr.log.V(5).Info("Response received from hook", "response", *res)

	for i := range res.EnvVars {
		obj.AddEnvVar(addEnvsPrefix+res.EnvVars[i].Name, &res.EnvVars[i])
	}

	if res.Status == nil {
		return nil
	}

	sm := obj.GetStatusManager()
	for i := range res.Status.Conditions {
		sm.SetCondition(&res.Status.Conditions[i])
	}
	for k, v := range res.Status.Annotations {
		if err := sm.SetAnnotation(k, v); err != nil {
			return err
		}
	}

	return nil
}

func (hr *hookReconciler) Finalize(ctx context.Context, obj reconciler.Object) error {
	hr.log.V(1).Info("Finalizing at hook", "obj", obj)

	if _, err := hr.requestHook(ctx, hookv1.OperationFinalize, obj); err != nil {
		return err
	}

	return nil
}

func (hr *hookReconciler) requestHook(ctx context.Context, operation hookv1.Operation, obj reconciler.Object) (*hookv1.HookResponse, error) {
	r := &hookv1.HookRequest{
		Object: commonv1alpha1.Reference{
			APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
		},
		Operation: operation,
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("could not marshal hook request: %w", err)
	}

	req, err := http.NewRequest("POST", hr.url, bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("could create hook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not execute hook request to %s: %w", hr.url, err)
	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		// Try to read any error message
		b, err := io.ReadAll(res.Body)
		reserr := ""
		if err != nil {
			reserr = "(could not read error response from hook)"
		} else {
			reserr = string(b)
		}
		return nil, fmt.Errorf("hook request at %s returned %d: %s", hr.url, res.StatusCode, reserr)
	}

	// Finalize do not expect data returned
	if operation == hookv1.OperationFinalize {
		return nil, nil
	}

	hres := &hookv1.HookResponse{}
	err = json.NewDecoder(res.Body).Decode(hres)
	if err != nil {
		return nil, fmt.Errorf("hook response from %s could not be parsed: %w", hr.url, err)
	}

	return hres, nil
}

func (hr *hookReconciler) IsReconciler() bool {
	// For now all hooks are reconcilers
	return true
}

func (hr *hookReconciler) IsFinalizer() bool {
	return hr.isFinalizer
}
