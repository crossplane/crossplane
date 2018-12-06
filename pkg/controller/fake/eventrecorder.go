package fake

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// FakeEventRecorder Kubernetes events recorder
type FakeEventRecorder struct {
	record.EventRecorder

	MockEvent           func(object runtime.Object, eventtype, reason, message string)
	MockEventf          func(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{})
	MockPastEventf      func(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{})
	MockAnnotatedEventf func(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{})
}

// The resulting event will be created in the same namespace as the reference object.
func (f *FakeEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	f.MockEvent(object, eventtype, reason, message)
}

// Eventf is just like Event, but with Sprintf for the message field.
func (f *FakeEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	f.MockEventf(object, eventtype, reason, messageFmt, args)
}

// PastEventf is just like Eventf, but with an option to specify the event's 'timestamp' field.
func (f *FakeEventRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
	f.MockPastEventf(object, timestamp, eventtype, reason, messageFmt, args)
}

// AnnotatedEventf is just like eventf, but with annotations attached
func (f *FakeEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	f.AnnotatedEventf(object, annotations, eventtype, reason, messageFmt, args)
}

func NewNilEventRecorder() *FakeEventRecorder {
	return &FakeEventRecorder{
		MockEvent: func(object runtime.Object, eventtype, reason, message string) {
		},
		MockEventf: func(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
		},
		MockPastEventf: func(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
		},
		MockAnnotatedEventf: func(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
		},
	}
}
