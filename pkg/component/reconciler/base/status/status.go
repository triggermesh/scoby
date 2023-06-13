// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-logr/logr"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
	"github.com/triggermesh/scoby/pkg/component/reconciler/base/crd"
)

// Time is a helper wrap around time.Now that
// enables us to write tests.
// +kubebuilder:object:generate=false
type Time interface {
	Now() metav1.Time
}

type realTime struct{}

func (realTime) Now() metav1.Time { return metav1.NewTime(time.Now()) }

type statusManagerFactory struct {
	flag      crd.StatusFlag
	happyCond string
	conds     map[string]struct{}

	time  Time
	log   logr.Logger
	mutex sync.RWMutex
}

func NewStatusManagerFactory(crdv *apiextensionsv1.CustomResourceDefinitionVersion, happyCond string, conditionSet []string, log logr.Logger) reconciler.StatusManagerFactory {
	smf := &statusManagerFactory{
		flag: crd.CRDStatusFlag(crdv),
		time: realTime{},
		log:  log,
	}

	smf.updateConditionSet(happyCond, conditionSet...)

	return smf
}

func (smf *statusManagerFactory) UpdateConditionSet(happyCond string, conditions ...string) {
	smf.mutex.Lock()
	defer smf.mutex.Unlock()
	smf.updateConditionSet(happyCond, conditions...)
}

func (smf *statusManagerFactory) updateConditionSet(happyCond string, conditions ...string) {

	smf.happyCond = happyCond

	conds := make(map[string]struct{}, len(conditions))
	for _, c := range conditions {
		conds[c] = struct{}{}
	}

	// if happy condition is missing from the set, add it.
	if _, ok := conds[happyCond]; !ok {
		conds[happyCond] = struct{}{}
	}

	smf.conds = conds
}

func (smf *statusManagerFactory) ForObject(object *unstructured.Unstructured) reconciler.StatusManager {
	smf.mutex.RLock()
	defer smf.mutex.RUnlock()

	sm := &statusManager{
		object:             object,
		happyConditionType: smf.happyCond,
		conditionTypes:     smf.conds,
		flag:               smf.flag,

		time: smf.time,
		log:  smf.log.WithName(object.GroupVersionKind().String()),
	}

	return sm
}

type statusManager struct {
	object *unstructured.Unstructured

	// Type for the condition that summarizes
	// happiness for the object.
	happyConditionType string

	// Condition types defined for the object,
	// including the happy condition type.
	conditionTypes map[string]struct{}

	// CRD information that informs about
	// the status capabilities.
	flag crd.StatusFlag

	time Time
	log  logr.Logger
	m    sync.RWMutex
}

// creates the object["status"] element
func (sm *statusManager) ensureStatusRoot() {
	if sm.object.Object == nil {
		sm.object.Object = map[string]interface{}{}
	}

	if _, ok := sm.object.Object["status"]; !ok {
		sm.object.Object["status"] = map[string]interface{}{}
	}
}

func (sm *statusManager) sanitizeConditions() {
	// When no flags set there status at the object's CRD.
	if !sm.flag.AllowConditions() {
		sm.log.V(2).Info("Skipping conditions: not supported by the CRD")
		return
	}

	sm.ensureStatusRoot()
	sm.log.V(2).Info("Ensuring status conditions")

	typedStatus := sm.object.Object["status"].(map[string]interface{})
	ecs, ok := typedStatus["conditions"]
	if !ok {
		ecs = make([]interface{}, 0, len(sm.conditionTypes))
		typedStatus["conditions"] = ecs
	}
	existingConditions := ecs.([]interface{})

	// make sure all expected conditions are listed at the current status
	// by moving them to the front of the slice keeping the index.
	i := 0
	for _, ec := range existingConditions {
		// We expect each expected condition to be
		// map[string]string. The casting must be done
		// for the map and then for the value
		c, ok := ec.(map[string]interface{})
		if !ok {
			continue
		}

		// Expect that the condition has a type element.
		cType, ok := c["type"]
		if !ok {
			continue
		}

		// Expect that the type value is a string.
		csType, ok := cType.(string)
		if !ok {
			continue
		}

		// Type must be one of the condition types.
		if _, ok = sm.conditionTypes[csType]; !ok {
			continue
		}

		// Current condition is valid, keep it and increase
		// counter.
		existingConditions[i] = ec
		i++

		// if the condition is valid track readiness, but don't let
		// the readiness vote on itself, only dependent conditions
		if csType == sm.happyConditionType {
			continue
		}
	}

	// if there are conditions that we do not expect at the tail, remove them.
	if i != len(sm.conditionTypes)-1 {
		existingConditions = existingConditions[:i]
	}

	// some expected conditions might be missing, or some not expected conditions
	// might be present
	if len(sm.conditionTypes) != len(existingConditions) {
		// Use the same last transition time for all conditions
		tt := sm.time.Now()

		for k := range sm.conditionTypes {
			found := false
			for i = range existingConditions {
				// no need to validate casting with an ok parameters since
				// this same casting was done at the previous loop block.
				c := existingConditions[i].(map[string]interface{})
				if c["type"] == k {
					found = true
					break
				}
			}

			if found {
				continue
			}

			existingConditions = append(existingConditions,
				map[string]interface{}{
					"type":               k,
					"status":             string(metav1.ConditionUnknown),
					"lastTransitionTime": tt.UTC().Format(time.RFC3339),
					"reason":             commonv1alpha1.ConditionReasonUnknown,
					"message":            "",
				})
		}
	}

	sort.Slice(existingConditions, func(i, j int) bool {
		tci := existingConditions[i].(map[string]interface{})
		tcj := existingConditions[j].(map[string]interface{})
		return tci["type"].(string) < tcj["type"].(string)
	})

	typedStatus["conditions"] = existingConditions
}

func (sm *statusManager) GetCondition(conditionType string) *commonv1alpha1.Condition {
	if !sm.flag.AllowConditions() {
		return nil
	}

	// make sure conditions are available.
	sm.sanitizeConditions()

	existingConditions, _, _ := unstructured.NestedSlice(sm.object.Object, "status", "conditions")

	for i := range existingConditions {
		c, ok := existingConditions[i].(map[string]interface{})
		if !ok {
			continue
		}

		cType, ok := c["type"]
		if !ok {
			continue
		}

		// Expect that the type value is a string.
		csType, ok := cType.(string)
		if !ok {
			continue
		}

		if csType == conditionType {
			// This is the condition that we need to return

			t, err := time.Parse(time.RFC3339, c["lastTransitionTime"].(string))
			if err != nil {
				sm.log.Error(err, "could not parse condition lastTransitionTime", "lastTransitionTime", c["lastTransitionTime"])
			}

			return &commonv1alpha1.Condition{
				Type:               conditionType,
				Message:            c["message"].(string),
				LastTransitionTime: metav1.NewTime(t),
				Status:             metav1.ConditionStatus(c["status"].(string)),
				Reason:             c["reason"].(string),
			}
		}
	}

	sm.log.V(2).Info("Status condition not found", "type", conditionType)
	return nil
}

// SanitizeConditions makes sure the set of expected conditions exist with
// default values, not overwritting the existing ones and removing any that
// should not exist.
func (sm *statusManager) SanitizeConditions() {
	if !sm.flag.AllowConditions() {
		return
	}

	sm.m.Lock()
	defer sm.m.Unlock()

	sm.sanitizeConditions()
}

func (sm *statusManager) SetCondition(condition *commonv1alpha1.Condition) {
	if !sm.flag.AllowConditions() {
		return
	}

	sm.m.Lock()
	defer sm.m.Unlock()

	// make sure conditions are available.
	sm.sanitizeConditions()

	sm.setCondition(condition)
}

func (sm *statusManager) setCondition(condition *commonv1alpha1.Condition) {
	typedStatus := sm.object.Object["status"].(map[string]interface{})

	ecs, ok := typedStatus["conditions"]
	if !ok {
		ecs = make([]interface{}, 0, len(sm.conditionTypes))
		typedStatus["conditions"] = ecs
	}
	existingConditions := ecs.([]interface{})

	found := false
	for i := range existingConditions {
		c, ok := existingConditions[i].(map[string]interface{})
		if !ok {
			continue
		}

		cType, ok := c["type"]
		if !ok {
			continue
		}

		// Expect that the type value is a string.
		csType, ok := cType.(string)
		if !ok {
			continue
		}

		if csType == condition.Type {
			// This is the condition that we need to set
			c["message"] = condition.Message
			c["lastTransitionTime"] = sm.time.Now()
			c["status"] = string(condition.Status)
			c["reason"] = condition.Reason

			found = true
			break
		}
	}

	if !found {
		sm.log.Error(errors.New("condition type was not found at the object's condition set"), "condition type not found", "type", condition.Type)
		return
	}

	sm.updateConditionHappiness()
}

func (sm *statusManager) updateConditionHappiness() {
	if !sm.flag.AllowConditions() {
		return
	}

	typedStatus := sm.object.Object["status"].(map[string]interface{})

	ecs, ok := typedStatus["conditions"]
	if !ok {
		ecs = make([]interface{}, 0, len(sm.conditionTypes))
		typedStatus["conditions"] = ecs
	}
	conditions := ecs.([]interface{})

	happyConditionIndex := -1
	happyStatus := string(metav1.ConditionTrue)
	happyReason := commonv1alpha1.ConditionReasonAllTrue

	for i := range conditions {
		c, ok := conditions[i].(map[string]interface{})
		if !ok {
			sm.log.Error(errors.New("condition cannot be parsed"), "Could not process condition happiness", "condition", conditions[i])
			continue
		}

		cType, ok := c["type"]
		if !ok {
			sm.log.Error(errors.New("condition does not have a type"), "Could not process condition happiness", "condition", c)
			continue
		}

		// Expect that the type value is a string.
		csType, ok := cType.(string)
		if !ok {
			sm.log.Error(errors.New("condition type is not a string"), "Could not process condition happiness", "type", cType)
			continue
		}

		if csType == sm.happyConditionType {
			// Happiness does not vote on itself
			happyConditionIndex = i
			continue
		}

		cStatus, ok := c["status"]
		if !ok {
			sm.log.Error(errors.New("condition does not have a status entry"), "Could not process condition happiness", "condition", c)
			continue
		}

		sStatus, ok := cStatus.(string)
		if !ok {
			sm.log.Error(errors.New("condition status not expected"), "Could not process condition happiness", "status", cStatus)
			continue
		}

		if sStatus == string(metav1.ConditionFalse) && happyStatus != string(metav1.ConditionFalse) {
			happyStatus = string(metav1.ConditionFalse)
			happyReason = commonv1alpha1.ConditionReasonNotAllTrue
			continue
		}

		if sStatus == string(metav1.ConditionUnknown) && happyStatus != string(metav1.ConditionUnknown) {
			happyStatus = string(metav1.ConditionUnknown)
			happyReason = commonv1alpha1.ConditionReasonUnknown
		}
	}

	if happyConditionIndex == -1 {
		sm.log.Error(errors.New("conditions do not have the happy condition item"), "Could not process condition happiness", "happy", sm.happyConditionType)
		return
	}

	hc := conditions[happyConditionIndex].(map[string]interface{})
	if hc["status"] != happyStatus || hc["reason"] != happyReason {
		hc["status"] = happyStatus
		hc["reason"] = happyReason
		hc["lastTransitionTime"] = sm.time.Now()
	}
}

func (sm *statusManager) GetObservedGeneration() int64 {
	if !sm.flag.AllowObservedGeneration() {
		return 0
	}

	sm.m.RLock()
	defer sm.m.RUnlock()

	g, _, _ := unstructured.NestedInt64(sm.object.Object, "status", "observedGeneration")
	return g
}

func (sm *statusManager) SetObservedGeneration(g int64) {
	if !sm.flag.AllowObservedGeneration() {
		return
	}

	sm.m.Lock()
	defer sm.m.Unlock()

	sm.ensureStatusRoot()
	typedStatus := sm.object.Object["status"].(map[string]interface{})

	og, ok := typedStatus["observedGeneration"]
	if ok {
		if iog := og.(int64); iog == g {
			return
		}
	}

	typedStatus["observedGeneration"] = g
}

func (sm *statusManager) GetAddressURL() string {
	if !sm.flag.AllowAddressURL() {
		return ""
	}

	sm.m.Lock()
	defer sm.m.Unlock()

	sm.ensureStatusRoot()
	typedStatus := sm.object.Object["status"].(map[string]interface{})

	address, ok := typedStatus["address"]
	if !ok {
		return ""
	}

	typedAddress, ok := address.(map[string]interface{})
	if !ok {
		return ""
	}

	url, ok := typedAddress["url"]
	if !ok {
		return ""
	}

	typedUrl, ok := url.(string)
	if !ok {
		return ""
	}

	return typedUrl
}

func (sm *statusManager) SetAddressURL(url string) {
	if !sm.flag.AllowAddressURL() {
		return
	}

	sm.m.Lock()
	defer sm.m.Unlock()

	sm.ensureStatusRoot()
	typedStatus := sm.object.Object["status"].(map[string]interface{})

	typedStatus["address"] = map[string]string{
		"url": url,
	}
}

func (sm *statusManager) SetValue(value interface{}, path ...string) error {
	sm.m.Lock()
	defer sm.m.Unlock()

	sm.ensureStatusRoot()
	return unstructured.SetNestedField(sm.object.Object, value, path...)
}

func (sm *statusManager) SetAnnotation(key, value string) error {
	sm.m.Lock()
	defer sm.m.Unlock()

	sm.ensureStatusRoot()

	return sm.setAnnotation(key, value)
}

func (sm *statusManager) setAnnotation(key, value string) error {
	typedStatus := sm.object.Object["status"].(map[string]interface{})

	annotations, ok := typedStatus["annotations"]
	if !ok {
		typedStatus["annotations"] = map[string]string{
			key: value,
		}
		return nil
	}

	typedAnnotations, ok := annotations.(map[string]interface{})
	if !ok {
		return errors.New("unexpected type for status.annotations")
	}

	typedAnnotations[key] = value
	return nil
}

// Merge an incoming unstructured into the existing status, taking care of
func (sm *statusManager) Merge(status map[string]interface{}) error {
	sm.m.Lock()
	defer sm.m.Unlock()

	sm.ensureStatusRoot()

	// incoming root element might be present or not.
	ost, ok := status["status"]
	if !ok {
		ost = status
	}

	st, ok := ost.(map[string]interface{})
	if !ok {
		return fmt.Errorf("incoming status cannot be converted into map[string]interface{}: %+v", ost)
	}

	// Incoming status elements replace existing, but for conditions and annotations.
	// Conditions will

	for k, v := range st {

		switch k {
		case "conditions":
			// iterate each condition and set individually.

			varr, ok := v.([]interface{})
			if !ok {
				return fmt.Errorf("incoming status conditions cannot be converted to []interface{}: %+v", v)
			}

			for _, vitem := range varr {
				c, ok := vitem.(map[string]interface{})
				if !ok {
					return fmt.Errorf("incoming status conditions cannot be converted to []interface{}: %+v", vitem)
				}

				condition := &commonv1alpha1.Condition{}

				condition.Type, ok = c["type"].(string)
				if !ok {
					return fmt.Errorf("incoming condition type as string not found: %+v", c["type"])
				}

				cstatus, ok := c["status"].(string)
				if !ok {
					return fmt.Errorf("incoming condition status as string not found: %+v", c["status"])
				}
				condition.Status = metav1.ConditionStatus(cstatus)

				condition.Reason, _ = c["reason"].(string)
				condition.Message, _ = c["message"].(string)

				sm.setCondition(condition)
			}

		case "annotations":
			annotations, ok := v.(map[string]interface{})
			if !ok {
				return fmt.Errorf("incoming status annotations cannot be converted into map[string]interface{}: %+v", v)
			}

			for k, v := range annotations {
				if err := sm.setAnnotation(k, v.(string)); err != nil {
					return err
				}
			}

		default:
			// overwrite if existing
			if err := sm.SetValue(v, "status", k); err != nil {
				return err
			}
		}
	}

	return nil
}
