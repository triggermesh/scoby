package crd

import (
	"errors"
	"sort"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ConditionReasonUnknown    = "UNKNOWN"
	ConditionReasonAllTrue    = "CONDITIONSOK"
	ConditionReasonNotAllTrue = "CONDITIONSNOTOK"
)

// Time is a helper wrap around time.Now that
// enables us to write tests.
// +kubebuilder:object:generate=false
type Time interface {
	Now() metav1.Time
}

type realTime struct{}

func (realTime) Now() metav1.Time { return metav1.NewTime(time.Now()) }

type StatusManagerFactory interface {
	ForObject(object *unstructured.Unstructured) StatusManager
}

type statusManagerFactory struct {
	flag      StatusFlag
	happyCond string
	conds     map[string]struct{}

	time Time
	log  logr.Logger
}

func NewStatusManagerFactory(flag StatusFlag, happyCond string, conds map[string]struct{}, log logr.Logger) StatusManagerFactory {
	return &statusManagerFactory{
		flag:      flag,
		happyCond: happyCond,
		conds:     conds,
		time:      realTime{},
		log:       log,
	}
}

type StatusManager interface {
	Init()
	SetObservedGeneration(int64)
	SetCondition(typ string, status metav1.ConditionStatus, reason, message string)
}

func (smf *statusManagerFactory) ForObject(object *unstructured.Unstructured) StatusManager {
	sm := &statusManager{
		object:             object,
		happyConditionType: smf.happyCond,
		conditionTypes:     smf.conds,
		flag:               smf.flag,

		time: smf.time,
		log:  smf.log,
	}

	sm.sanitizeConditions()

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
	flag StatusFlag

	time Time
	log  logr.Logger
}

func (sm *statusManager) Init() {
	sm.sanitizeConditions()
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
	if sm.flag == 0 {
		sm.log.V(2).Info("Skipping status: subresource is not present at CRD")
		return
	}

	sm.ensureStatusRoot()
	typedStatus := sm.object.Object["status"].(map[string]interface{})

	if sm.flag.AllowObservedGeneration() {
		sm.log.V(2).Info("Writing status observedGeneration")
		_, ok := typedStatus["observedGeneration"]
		if !ok {
			typedStatus["observedGeneration"] = int64(0)
		}
	}

	sm.log.V(2).Info("Writing status conditions")

	ecs, ok := typedStatus["conditions"]
	if !ok {
		ecs = make([]interface{}, 0, len(sm.conditionTypes))
		typedStatus["conditions"] = ecs
	}
	existingConditions := ecs.([]interface{})

	// make sure all expected conditions are listed at the current status
	// by moving them to the front of the slice keeping the index.
	i := 0
	// also keep track of the happiness by using the other existing
	// consitions statuses.
	happyStatus := metav1.ConditionTrue

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

		// look for the status element of this condition
		s, ok := c["status"]
		if !ok {
			continue
		}

		// status value must be a string
		ss, ok := s.(string)
		if !ok {
			continue
		}

		switch metav1.ConditionStatus(ss) {
		case metav1.ConditionFalse:
			if happyStatus != metav1.ConditionFalse {
				happyStatus = metav1.ConditionFalse
			}
		case metav1.ConditionUnknown:
			if happyStatus == metav1.ConditionTrue {
				happyStatus = metav1.ConditionUnknown
			}
		}
	}

	// if there are conditions that we do not expect at the tail, remove them.
	if i != len(sm.conditionTypes)-1 {
		existingConditions = existingConditions[:i]
	}

	// some expected conditions might be missing, or some not expected conditions
	// might be present
	if len(sm.conditionTypes) != len(existingConditions) {
		// new elements are going to be added, set global readiness to unknown
		if happyStatus == metav1.ConditionTrue {
			happyStatus = metav1.ConditionUnknown
		}

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
					"reason":             ConditionReasonUnknown,
					"message":            "",
				})
		}
	}

	sort.Slice(existingConditions, func(i, j int) bool {
		tci := existingConditions[i].(map[string]interface{})
		tcj := existingConditions[j].(map[string]interface{})
		return tci["type"].(string) < tcj["type"].(string)
	})

	// TODO Adjust hapiness.

	typedStatus["conditions"] = existingConditions
}

func (sm *statusManager) SetCondition(typ string, status metav1.ConditionStatus, reason, message string) {
	if !sm.flag.AllowConditions() {
		return
	}

	sm.ensureStatusRoot()
	typedStatus := sm.object.Object["status"].(map[string]interface{})

	ecs, ok := typedStatus["conditions"]
	if !ok {
		ecs = make([]interface{}, 0, len(sm.conditionTypes))
		typedStatus["conditions"] = ecs
	}
	existingConditions := ecs.([]interface{})

	found := false
	for i := range existingConditions {
		sm.log.Info("Debugdeleteme iterating condition 0", "i", i)
		c, ok := existingConditions[i].(map[string]interface{})
		if !ok {
			continue
		}
		sm.log.Info("Debugdeleteme iterating condition 1", "i", i)

		cType, ok := c["type"]
		if !ok {
			continue
		}
		sm.log.Info("Debugdeleteme iterating condition 2", "i", i)

		// Expect that the type value is a string.
		csType, ok := cType.(string)
		if !ok {
			continue
		}
		sm.log.Info("Debugdeleteme iterating condition 3", "i", i)

		if csType == typ {
			sm.log.Info("Debugdeleteme iterating BINGO!", "i", i)
			// This is the condition that we need to set
			c["message"] = message
			c["lastTransitionTime"] = sm.time.Now()
			c["status"] = status
			c["reason"] = reason

			found = true
			break
		}
	}

	if !found {
		sm.log.Error(errors.New("condition type was not found at the object's condition set"), "condition type not found", "type", typ)
		return
	}

	sm.sanitizeConditions()
}

func (sm *statusManager) SetObservedGeneration(g int64) {
	if !sm.flag.AllowObservedGeneration() {
		return
	}

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
