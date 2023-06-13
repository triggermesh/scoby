package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	hookv1 "github.com/triggermesh/scoby/pkg/apis/hook/v1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
)

type hookReconciler struct {
	url        string
	conditions []commonv1alpha1.ConditionsFromHook

	isPreReconciler bool
	isFinalizer     bool

	log logr.Logger
}

func New(h *commonv1alpha1.Hook, url string, conditions []commonv1alpha1.ConditionsFromHook, log logr.Logger) reconciler.HookReconciler {
	hr := &hookReconciler{
		url: url,

		isPreReconciler: h.Capabilities.IsPreReconciler(),
		isFinalizer:     h.Capabilities.IsFinalizer(),

		conditions: conditions,

		log: log,
	}

	return hr
}

func (hr *hookReconciler) PreReconcile(ctx context.Context, obj reconciler.Object, candidates *map[string]*unstructured.Unstructured) *reconciler.HookError {
	hr.log.V(1).Info("Pre-reconciling at hook", "obj", obj)

	err := hr.preReconcileHTTPRequest(ctx, obj, candidates)

	return err
}

func (hr *hookReconciler) preReconcileHTTPRequest(ctx context.Context, obj reconciler.Object, candidates *map[string]*unstructured.Unstructured) *reconciler.HookError {
	uobj, ok := obj.AsKubeObject().(*unstructured.Unstructured)
	if !ok {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("could not parse object into unstructured: %s", obj.GetName()),
		}
	}

	b, err := json.Marshal(&hookv1.HookRequest{
		Object:   *uobj,
		Phase:    hookv1.PhasePreReconcile,
		Children: *candidates,
	})
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("could not marshal hook request: %w", err),
		}
	}

	req, err := http.NewRequest("POST", hr.url, bytes.NewBuffer(b))
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("could create hook request: %w", err),
		}
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("could not execute hook request to %s: %w", hr.url, err),
		}
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
		return &reconciler.HookError{
			// Do not mark as permanent to retry the hook
			Permanent: false,
			Continue:  false,
			Err:       fmt.Errorf("hook request at %s returned %d: %s", hr.url, res.StatusCode, reserr),
		}
	}

	hres := &hookv1.HookResponse{}
	err = json.NewDecoder(res.Body).Decode(hres)
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("hook response from %s could not be parsed: %w", hr.url, err),
		}
	}

	hr.log.V(5).Info("Response received from hook", "response", *res)

	if hres.Error != nil {
		he := &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       errors.New(hres.Error.Message),
		}

		if hres.Error.Permanent != nil {
			he.Permanent = *hres.Error.Permanent
		}
		if hres.Error.Continue != nil {
			he.Continue = *hres.Error.Continue
		}

		return he
	}

	if hres.Children != nil && len(hres.Children) != 0 {
		*candidates = hres.Children
	}

	if hres.Status == nil {
		return nil
	}

	sm := obj.GetStatusManager()
	err = sm.Merge(hres.Status)
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("hook response could not merge reconciled object status: %w", err),
		}
	}

	return nil
}

func (hr *hookReconciler) IsPreReconciler() bool {
	return hr.isPreReconciler
}

func (hr *hookReconciler) IsFinalizer() bool {
	return hr.isFinalizer
}

func (hr *hookReconciler) Finalize(ctx context.Context, obj reconciler.Object) *reconciler.HookError {
	hr.log.V(1).Info("Finalizing at hook", "obj", obj)

	err := hr.finalizerHTTPRequest(ctx, obj)
	return err
}

func (hr *hookReconciler) finalizerHTTPRequest(ctx context.Context, obj reconciler.Object) *reconciler.HookError {
	uobj, ok := obj.AsKubeObject().(*unstructured.Unstructured)
	if !ok {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("could not parse object into unstructured: %s", obj.GetName()),
		}
	}

	b, err := json.Marshal(&hookv1.HookRequest{
		Object: *uobj,
		Phase:  hookv1.PhaseFinalize,
	})
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("could not marshal hook request: %w", err),
		}
	}

	req, err := http.NewRequest("POST", hr.url, bytes.NewBuffer(b))
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("could create hook request: %w", err),
		}
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("could not execute hook request to %s: %w", hr.url, err),
		}
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
		return &reconciler.HookError{
			// Do not mark as permanent to retry the hook
			Permanent: false,
			Continue:  false,
			Err:       fmt.Errorf("hook request at %s returned %d: %s", hr.url, res.StatusCode, reserr),
		}
	}

	hres := &hookv1.HookResponse{}
	err = json.NewDecoder(res.Body).Decode(hres)
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("hook response from %s could not be parsed: %w", hr.url, err),
		}
	}

	hr.log.V(5).Info("Response received from hook", "response", *res)

	if hres.Error != nil {
		he := &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       errors.New(hres.Error.Message),
		}

		if hres.Error.Permanent != nil {
			he.Permanent = *hres.Error.Permanent
		}
		if hres.Error.Continue != nil {
			he.Continue = *hres.Error.Continue
		}

		return he
	}

	if hres.Status == nil {
		return nil
	}

	sm := obj.GetStatusManager()
	err = sm.Merge(hres.Status)
	if err != nil {
		return &reconciler.HookError{
			Permanent: true,
			Continue:  false,
			Err:       fmt.Errorf("hook response could not merge reconciled object status: %w", err),
		}
	}

	return nil
}
