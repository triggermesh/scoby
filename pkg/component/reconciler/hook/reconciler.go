package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	hookv1 "github.com/triggermesh/scoby/pkg/apis/hook/v1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
)

const (
	addEnvsPrefix = "$hook."
)

type hookReconciler struct {
	url        string
	conditions []commonv1alpha1.ConditionsFromHook

	isPreReconciler  bool
	isPostReconciler bool
	isFinalizer      bool

	log logr.Logger
}

func New(h *commonv1alpha1.Hook, url string, conditions []commonv1alpha1.ConditionsFromHook, log logr.Logger) reconciler.HookReconciler {
	hr := &hookReconciler{
		url: url,

		isPreReconciler:  h.Capabilities.IsPreReconciler(),
		isPostReconciler: h.Capabilities.IsPostReconciler(),
		isFinalizer:      h.Capabilities.IsFinalizer(),

		conditions: conditions,

		log: log,
	}

	return hr
}

func (hr *hookReconciler) Reconcile(ctx context.Context, obj reconciler.Object) error {
	hr.log.V(1).Info("Reconciling at hook", "obj", obj)

	res, err := hr.requestHook(ctx, commonv1alpha1.HookCapabilityPreReconcile, obj)
	if err == nil {
		hr.log.V(5).Info("Response received from hook", "response", *res)

		if res.Workload != nil && res.Workload.PodSpec != nil &&
			len(res.Workload.PodSpec.Containers) > 0 {
			ev := res.Workload.PodSpec.Containers[0].Env
			for i := range ev {
				obj.AddEnvVar(addEnvsPrefix+ev[i].Name, &ev[i])
			}
		}
	}

	if upErr := hr.updateStatus(obj, res, err); upErr != nil {
		hr.log.Error(upErr, "could not update the object's status from the hook", "object", obj)
	}

	return err
}

func (hr *hookReconciler) Finalize(ctx context.Context, obj reconciler.Object) error {
	hr.log.V(1).Info("Finalizing at hook", "obj", obj)

	res, err := hr.requestHook(ctx, commonv1alpha1.HookCapabilityFinalize, obj)
	if err == nil {
		return nil
	}

	if upErr := hr.updateStatus(obj, res, err); upErr != nil {
		hr.log.Error(upErr, "could not update the object's status from the hook", "object", obj)
	}

	return err
}

func (hr *hookReconciler) requestHook(ctx context.Context, phase commonv1alpha1.HookCapability, obj reconciler.Object) (*hookv1.HookResponse, error) {
	r := &hookv1.HookRequest{
		Object: commonv1alpha1.Reference{
			APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
		},
		Phase: phase,
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
	if phase == commonv1alpha1.HookCapabilityFinalize {
		return nil, nil
	}

	hres := &hookv1.HookResponse{}
	err = json.NewDecoder(res.Body).Decode(hres)
	if err != nil {
		return nil, fmt.Errorf("hook response from %s could not be parsed: %w", hr.url, err)
	}

	return hres, nil
}

func (hr *hookReconciler) IsPreReconciler() bool {
	return hr.isPreReconciler
}

func (hr *hookReconciler) IsPostReconciler() bool {
	return hr.isPostReconciler
}

func (hr *hookReconciler) IsFinalizer() bool {
	return hr.isFinalizer
}

func (hr *hookReconciler) updateStatus(obj reconciler.Object, res *hookv1.HookResponse, hookErr error) error {
	sm := obj.GetStatusManager()

	// Informed types keep track of conditions informed from the hook, those
	// not informed will be defaulted next.
	informedTypes := []string{}

	if res != nil {
		for i := range res.Status.Conditions {
			sm.SetCondition(&res.Status.Conditions[i])
			informedTypes = append(informedTypes, res.Status.Conditions[i].Type)
		}
		for k, v := range res.Status.Annotations {
			if err := sm.SetAnnotation(k, v); err != nil {
				return err
			}
		}
	}

	condReason := "UNKNOWN"
	condMessage := ""
	if hookErr != nil {
		condReason = "HOOKERROR"
		condMessage = hookErr.Error()
	}

	// Fill missing statuses with unknown conditions.
	for _, c := range hr.conditions {
		informed := false
		for _, it := range informedTypes {
			if c.Type == it {
				informed = true
				break
			}
		}

		if informed {
			continue
		}

		sm.SetCondition(&commonv1alpha1.Condition{
			Type:               c.Type,
			Status:             metav1.ConditionUnknown,
			LastTransitionTime: metav1.Now(),
			Reason:             condReason,
			Message:            condMessage,
		})
	}

	return nil
}
