// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/rickb777/date/period"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	hookv1 "github.com/triggermesh/scoby/pkg/apis/hook/v1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
)

const defaultTimeout = time.Second * 15

var (
	_true    = true
	_false   = false
	ptrTrue  = &_true
	ptrFalse = &_false
)

type hookReconciler struct {
	url        string
	timeout    time.Duration
	conditions []commonv1alpha1.ConditionsFromHook

	isPreReconciler bool
	isFinalizer     bool

	log logr.Logger
	ffi *hookv1.FormFactorInfo
}

func New(h *commonv1alpha1.Hook, url string, conditions []commonv1alpha1.ConditionsFromHook, ffi *hookv1.FormFactorInfo, log logr.Logger) reconciler.HookReconciler {
	hr := &hookReconciler{
		url:     url,
		timeout: defaultTimeout,

		isPreReconciler: h.Capabilities.IsPreReconciler(),
		isFinalizer:     h.Capabilities.IsFinalizer(),

		conditions: conditions,

		ffi: ffi,
		log: log,
	}

	if h.Timeout != nil {
		p, err := period.Parse(*h.Timeout)
		if err != nil {
			log.Error(err, "hook timeout is not an ISO 8601 duration", "timeout", h.Timeout)
		} else {
			hr.timeout = p.DurationApprox()
		}
	}

	return hr
}

func (hr *hookReconciler) PreReconcile(ctx context.Context, obj reconciler.Object, candidates *map[string]*unstructured.Unstructured) *hookv1.HookResponseError {
	hr.log.V(1).Info("Pre-reconciling at hook", "obj", obj)

	return hr.preReconcileHTTPRequest(ctx, obj, candidates)
}

func (hr *hookReconciler) preReconcileHTTPRequest(ctx context.Context, obj reconciler.Object, candidates *map[string]*unstructured.Unstructured) *hookv1.HookResponseError {
	uobj, ok := obj.AsKubeObject().(*unstructured.Unstructured)
	if !ok {
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("could not parse object into unstructured: %s", obj.GetName()),
		}
	}

	b, err := json.Marshal(&hookv1.HookRequest{
		FormFactor: *hr.ffi,
		Object:     *uobj,
		Phase:      hookv1.PhasePreReconcile,
		Children:   *candidates,
	})
	if err != nil {
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("could not marshal hook request: %w", err),
		}
	}

	req, err := http.NewRequest("POST", hr.url, bytes.NewBuffer(b))
	if err != nil {
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("could create hook request: %w", err),
		}
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{
		Timeout: hr.timeout,
	}

	res, err := client.Do(req)
	if err != nil {
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("executing hook request to %s: %w", hr.url, err),
		}
	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		// Try to read any error message
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return &hookv1.HookResponseError{
				Permanent: ptrFalse,
				Continue:  ptrFalse,
				Err:       fmt.Errorf("hook response from %s returning %d could not be read: %w", hr.url, res.StatusCode, err),
			}
		}

		// Try to convert to an structured error.
		he := &hookv1.HookResponseError{}
		err = json.Unmarshal(b, he)

		// If the response does not contain an structured error treat it as a string.
		if err != nil {
			return &hookv1.HookResponseError{
				// Do not mark as permanent to retry the hook
				Permanent: ptrFalse,
				Continue:  ptrFalse,
				Err:       fmt.Errorf("hook request at %s returned %d: %s", hr.url, res.StatusCode, string(b)),
			}
		}

		return he
	}

	hres := &hookv1.HookResponse{}
	err = json.NewDecoder(res.Body).Decode(hres)
	switch {
	case err == io.EOF:
		// an empty response that does not mean error, but
		// noop from the hook, just return
		return nil

	case err != nil:
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("hook response from %s could not be parsed: %w", hr.url, err),
		}
	}

	hr.log.V(5).Info("Response received from hook", "response", *res)

	if hres.Children != nil && len(hres.Children) != 0 {
		*candidates = hres.Children
	}

	if hres.Object == nil {
		return nil
	}

	*uobj = *hres.Object

	return nil
}

func (hr *hookReconciler) IsPreReconciler() bool {
	return hr.isPreReconciler
}

func (hr *hookReconciler) IsFinalizer() bool {
	return hr.isFinalizer
}

func (hr *hookReconciler) Finalize(ctx context.Context, obj reconciler.Object) *hookv1.HookResponseError {
	hr.log.V(1).Info("Finalizing at hook", "obj", obj)

	err := hr.finalizerHTTPRequest(ctx, obj)
	return err
}

func (hr *hookReconciler) finalizerHTTPRequest(ctx context.Context, obj reconciler.Object) *hookv1.HookResponseError {
	uobj, ok := obj.AsKubeObject().(*unstructured.Unstructured)
	if !ok {
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("could not parse object into unstructured: %s", obj.GetName()),
		}
	}

	b, err := json.Marshal(&hookv1.HookRequest{
		FormFactor: *hr.ffi,
		Object:     *uobj,
		Phase:      hookv1.PhaseFinalize,
	})
	if err != nil {
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("could not marshal hook request: %w", err),
		}
	}

	req, err := http.NewRequest("POST", hr.url, bytes.NewBuffer(b))
	if err != nil {
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("could not create hook request: %w", err),
		}
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{
		Timeout: hr.timeout,
	}

	res, err := client.Do(req)
	if err != nil {
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("executing hook request to %s: %w", hr.url, err),
		}
	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		// Try to read any error message
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return &hookv1.HookResponseError{
				Permanent: ptrFalse,
				Continue:  ptrFalse,
				Err:       fmt.Errorf("hook response from %s returning %d could not be read: %w", hr.url, res.StatusCode, err),
			}
		}

		// Try to convert to an structured error.
		he := &hookv1.HookResponseError{}
		err = json.Unmarshal(b, he)

		// If the response does not contain an structured error treat it as a string.
		if err != nil {
			return &hookv1.HookResponseError{
				// Do not mark as permanent to retry the hook
				Permanent: ptrFalse,
				Continue:  ptrFalse,
				Err:       fmt.Errorf("hook request at %s returned %d: %s", hr.url, res.StatusCode, string(b)),
			}
		}

		return he
	}

	hres := &hookv1.HookResponse{}
	err = json.NewDecoder(res.Body).Decode(hres)
	switch {
	case err == io.EOF:
		// an empty response that does not mean error, but
		// noop from the hook, just return
		return nil

	case err != nil:
		return &hookv1.HookResponseError{
			Permanent: ptrTrue,
			Continue:  ptrFalse,
			Err:       fmt.Errorf("hook response from %s could not be parsed: %w", hr.url, err),
		}
	}

	hr.log.V(5).Info("Response received from hook", "response", *res)

	if hres.Object == nil {
		return nil
	}

	*uobj = *hres.Object

	return nil
}
