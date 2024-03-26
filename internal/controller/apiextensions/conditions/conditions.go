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

type Condition struct {
	xpv1.Condition
	EventType event.Type
}

func Normal(c xpv1.Condition) Condition {
	return Condition{
		Condition: c,
		EventType: event.TypeNormal,
	}
}

func Warning(c xpv1.Condition) Condition {
	return Condition{
		Condition: c,
		EventType: event.TypeWarning,
	}
}

type ConditionMap map[xpv1.ConditionType]Condition

type ConditionProcessor struct {
	Obj       *unstructured.Unstructured
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

func New(u *unstructured.Unstructured, opts ...Option) *ConditionProcessor {
	cp := &ConditionProcessor{
		Obj:      u,
		Recorder: event.NewNopRecorder(),
	}

	for _, o := range opts {
		o(cp)
	}

	return cp
}

func (cp ConditionProcessor) SetCondition(c Condition) {
	cp.SetConditions(ConditionMap{c.Type: c})
}

func (cp ConditionProcessor) SetConditions(cm ConditionMap, opts ...Option) {
	// TODO(dalton): test that this does not update the ConditionProcessor
	for _, o := range opts {
		o(&cp)
	}
	prev := GetConditions(cp.Obj)

	cmp := prev.Compare(cm)
	updated := len(cmp.New) > 0
	if updated {
		RecordConditions(cp.Obj, cp.Recorder, cmp.New)
	}

	var nm ConditionMap
	if cp.DropStale {
		nm = mergeMaps(cmp.Equal, cmp.New)
		if len(cmp.Old) > 0 {
			// old entries were dropped
			updated = true
		}
	} else {
		nm = mergeMaps(cmp.Old, cmp.Equal, cmp.New)
	}

	// if the nm does not contain a system condition, try to copy that condition
	// from the previous conditions
	for k := range systemConditionTypes {
		if _, ok := nm[k]; ok {
			continue
		}
		if c, ok := prev[k]; ok {
			nm[k] = c
		}
	}

	if updated {
		// only update if a change occurred to prevent needless reconciliations
		ForceSetConditions(cp.Obj, nm)
	}
}

func ForceSetConditions(u *unstructured.Unstructured, cm ConditionMap) {
	conditioned := xpv1.ConditionedStatus{}
	for _, c := range cm {
		conditioned.Conditions = append(conditioned.Conditions, c.Condition)
	}
	_ = fieldpath.Pave(u.Object).SetValue("status.conditions", conditioned.Conditions)
}

type CompareResult struct {
	New   ConditionMap
	Equal ConditionMap
	Old   ConditionMap
}

func (cm ConditionMap) Compare(other ConditionMap) CompareResult {
	res := CompareResult{
		New:   make(ConditionMap),
		Equal: make(ConditionMap),
		Old:   make(ConditionMap),
	}
	for k, a := range cm {
		if b, ok := other[k]; ok && a.Equal(b.Condition) {
			res.Equal[k] = a
		} else {
			res.Old[k] = a
		}
	}
	for k, b := range other {
		if a, ok := cm[k]; !ok || !b.Equal(a.Condition) {
			res.New[k] = b
		}
	}
	return res
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
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(u.Object).GetValueInto("status", &conditioned); err != nil {
		return make(ConditionMap)
	}
	m := make(ConditionMap)
	for _, c := range conditioned.Conditions {
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
	if err := fieldpath.Pave(xr.Object).GetValueInto("status.claimConditions", &claimConditionTypes); err != nil {
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

// mergeMaps merges n maps, if same key is found in both maps[i] and maps[i+1], the value from
// maps[i+1] will be used
func mergeMaps[K comparable, V any](maps ...map[K]V) map[K]V {
	nm := make(map[K]V)

	for _, m := range maps {
		for k, v := range m {
			nm[k] = v
		}
	}

	return nm
}
