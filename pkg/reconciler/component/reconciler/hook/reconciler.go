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

type hookReconciler struct {
	url string

	log logr.Logger
}

func New(h *commonv1alpha1.Hook, url string, log logr.Logger) reconciler.HookReconciler {

	hr := &hookReconciler{
		url: url,
		log: log,
	}

	return hr
}

func (hr *hookReconciler) Reconcile(ctx context.Context, obj reconciler.Object) error {
	hr.log.V(1).Info("Reconciling at hook", "obj", obj)
	r := &hookv1.HookRequest{
		Object: commonv1alpha1.Reference{
			APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
		},
		Operation: hookv1.OperationReconcile,
	}
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("could not marshal hook request: %w", err)
	}

	req, err := http.NewRequest("POST", hr.url, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("could create hook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not execute hook request to %s: %w", hr.url, err)
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
		return fmt.Errorf("hook request at %s returned %d: %s", hr.url, res.StatusCode, reserr)
	}

	hres := &hookv1.HookResponse{}
	err = json.NewDecoder(res.Body).Decode(hres)
	if err != nil {
		return fmt.Errorf("hook response from %s could not be parsed: %w", hr.url, err)
	}

	hr.log.V(5).Info("Response received from hook", "response", *hres)

	// request
	// if response contains status info, add it.
	// if response contains env var, add it.

	return nil
}

func (hr *hookReconciler) Finalize(ctx context.Context, obj reconciler.Object) error {
	hr.log.V(1).Info("Finalizing at hook", "obj", obj)

	return nil
}
