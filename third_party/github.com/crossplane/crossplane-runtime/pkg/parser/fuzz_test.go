package parser

import (
	"bytes"
	"context"
	"io"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func FuzzParse(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		objScheme := runtime.NewScheme()
		metaScheme := runtime.NewScheme()
		p := New(metaScheme, objScheme)
		r := io.NopCloser(bytes.NewReader(data))
		_, _ = p.Parse(context.Background(), r)
	})
}
