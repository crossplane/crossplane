package conditions

import (
	"errors"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	// conditions that should always be present and are controlled by the system
	systemConditionTypes = map[xpv1.ConditionType]struct{}{
		xpv1.TypeReady:  {},
		xpv1.TypeSynced: {},
	}
)

/*
TODO:
- setting conditions creates an object, object holds a recorder
	since the recorder will likely be default behavior, we want to do:
	instance.SetConditions(xr, []Conditions)
	not:
	conditions.SetConditions(xr, conditions, WithRecorder(r.recorder))
- break up the SetConditions function to reduce complexity but also abstract what we need
	to log changes to the claim object
*/

type Condition struct {
	xpv1.Condition
	EventType event.Type
}

type ConditionMap map[xpv1.ConditionType]Condition

// func: GetChangedConditions(u, prev) - returns any conditions that are new or need to be updated
// func: getChangedConditions(new, prev) - returns any conditions that are new or need to be updated
// func: IsSystemCondition(condition)
// func: SetConditions(force=bool)
// func: setConditions(obj, conditions) - completely wipes conditions, setting what is provided in arg

/*
prev := GetConditions()
changed := GetChangedConditions(prev, curr)
RecordConditions(claim, changed[only claim])
RecordConditions(composite, changed)
ForceSetConditions(composite)
SetClaimConditions(composite)
*/

type ConditionProcessor struct {
	Recorder  event.Recorder
	DropStale bool
}

type Option func(*ConditionProcessor)

// WithRecorder - record any new or updated conditions using the provided
// recorder.
func WithRecorder(r event.Recorder) Option {
	return func(cp *ConditionProcessor) {
		cp.Recorder = r
	}
}

// DropStale - Determines if old, non-system conditions are dropped when setting new ones.
func DropStale(b bool) Option {
	return func(cp *ConditionProcessor) {
		cp.DropStale = b
	}
}

func New(opts ...Option) *ConditionProcessor {
	cp := &ConditionProcessor{
		Recorder: event.NewNopRecorder(),
	}

	for _, o := range opts {
		o(cp)
	}

	return cp
}

func (cp ConditionProcessor) SetConditions(u *unstructured.Unstructured, cm ConditionMap, opts ...Option) {
	// TODO(dalton): test that this does not update the ConditionProcessor
	for _, o := range opts {
		o(&cp)
	}
	prev := GetConditions(u)

	upToDate := GetChangedConditions(prev, cm)
	// record the list of changed conditions
	RecordConditions(u, cp.Recorder, upToDate)
	for _, p := range prev {
		if _, ok := upToDate[p.Type]; ok {
			// a more up-to-date entry exists
			continue
		}
		if _, ok := systemConditionTypes[p.Type]; ok {
			// always keep an entry for system conditions
			upToDate[p.Type] = p
			continue
		}
		if !cp.DropStale {
			upToDate[p.Type] = p
		}
	}

	ForceSetConditions(u, upToDate)
}

func ForceSetConditions(u *unstructured.Unstructured, cm ConditionMap) {
	conditioned := xpv1.ConditionedStatus{}
	for _, c := range cm {
		conditioned.Conditions = append(conditioned.Conditions, c.Condition)
	}
	_ = fieldpath.Pave(u.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetChangedConditions will take the previous and current conditions. It will
// return a subset of conditions that match one of the following:
// - (new) exists in curr but not in prev
// - (updated) exists in both but has been updated in curr
func GetChangedConditions(prev, curr ConditionMap) ConditionMap {
	fresh := make(ConditionMap)
	for _, c := range curr {
		if p, ok := prev[c.Type]; ok && p.Condition.Equal(c.Condition) {
			continue
		}
		fresh[c.Type] = c
	}
	return fresh
}

func RecordConditions(obj runtime.Object, r event.Recorder, cm ConditionMap) {
	for _, c := range cm {
		switch c.EventType {
		case event.TypeWarning:
			r.Event(obj, event.Warning(event.Reason(c.Reason), errors.New(c.Message)))
		case event.TypeNormal:
			r.Event(obj, event.Normal(event.Reason(c.Reason), c.Message))
		}
	}
}

// SetClaimConditionTypes sets the composite's status.claimConditions to whatever is provided
// as the conditions arg. All existing conditions will be wiped.
func SetClaimConditionTypes(xr *composite.Unstructured, cm ConditionMap) {
	ts := []xpv1.ConditionType{}
	for t := range cm {
		ts = append(ts, t)
	}
	_ = fieldpath.Pave(xr.Object).SetValue("status.claimConditions", ts)
}

// GetConditions returns all items from the object's status.conditions.
func GetConditions(u *unstructured.Unstructured) ConditionMap {
	conditions := []xpv1.Condition{}
	if err := fieldpath.Pave(u.Object).GetValueInto("status.conditions", &conditions); err != nil {
		return make(ConditionMap)
	}
	m := make(ConditionMap)
	for _, c := range conditions {
		m[c.Type] = Condition{
			Condition: c,
		}
	}
	return m
}

// GetClaimConditions returns all conditions from the composite that apply to the claim.
func GetClaimConditions(xr *composite.Unstructured) ConditionMap {
	conditions := GetConditions(&xr.Unstructured)
	claimConditionTypes := []string{}
	if err := fieldpath.Pave(xr.Object).GetValueInto("status.claimConditions", claimConditionTypes); err != nil {
		return make(ConditionMap)
	}
	claimConditions := make(ConditionMap)
	for _, t := range claimConditionTypes {
		if c, ok := conditions[xpv1.ConditionType(t)]; ok {
			claimConditions[xpv1.ConditionType(t)] = c
		}
	}
	return claimConditions
}
