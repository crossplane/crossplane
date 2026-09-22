/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package render

import (
	"context"
	"slices"

	"github.com/crossplane/crossplane/v2/internal/xfn"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// RecordingRequiredResourcesFetcher is a required resource fetcher that records
// the requested resource selectors for later retrieval.
type RecordingRequiredResourcesFetcher struct {
	wrap xfn.RequiredResourcesFetcher

	record []*fnv1.ResourceSelector
}

// Fetch records the requested resource selector, then calls the wrapped fetcher
// and returns its results.
func (f *RecordingRequiredResourcesFetcher) Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	f.record = append(f.record, rs)
	return f.wrap.Fetch(ctx, rs)
}

// GetResourceSelectors returns the recorded resource selectors.
func (f *RecordingRequiredResourcesFetcher) GetResourceSelectors() []*fnv1.ResourceSelector {
	return slices.Clone(f.record)
}

// NewRecordingRequiredResourcesFetcher returns a recording resource fetcher
// that wraps the given resource fetcher.
func NewRecordingRequiredResourcesFetcher(wrap xfn.RequiredResourcesFetcher) *RecordingRequiredResourcesFetcher {
	return &RecordingRequiredResourcesFetcher{
		wrap: wrap,
	}
}

// RecordingRequiredSchemasFetcher is a required schema fetcher that records the
// requested schema selectors for later retrieval.
type RecordingRequiredSchemasFetcher struct {
	wrap xfn.RequiredSchemasFetcher

	record []*fnv1.SchemaSelector
}

// Fetch records the requested schema selector, then calls the wrapped fetcher
// and returns its results.
func (f *RecordingRequiredSchemasFetcher) Fetch(ctx context.Context, ss *fnv1.SchemaSelector) (*fnv1.Schema, error) {
	f.record = append(f.record, ss)
	return f.wrap.Fetch(ctx, ss)
}

// GetSchemaSelectors returns the recorded schema selectors.
func (f *RecordingRequiredSchemasFetcher) GetSchemaSelectors() []*fnv1.SchemaSelector {
	return slices.Clone(f.record)
}

// NewRecordingRequiredSchemasFetcher returns a recording schema fetcher that
// wraps the given schema fetcher.
func NewRecordingRequiredSchemasFetcher(wrap xfn.RequiredSchemasFetcher) *RecordingRequiredSchemasFetcher {
	return &RecordingRequiredSchemasFetcher{
		wrap: wrap,
	}
}
