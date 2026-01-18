/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

// Package cached implements a cached function runner.
package cached

import (
	"context"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/proto/fn/v1alpha1"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// A CacheMissReason indicates what caused a cache miss.
type CacheMissReason string

// Cache miss reasons.
const (
	ReasonNotCached       CacheMissReason = "NotCached"
	ReasonDeadlineExpired CacheMissReason = "DeadlineExpired"
	ReasonEmptyRequestTag CacheMissReason = "EmptyRequestTag"
	ReasonError           CacheMissReason = "Error"
)

// Metrics for the function response cache.
type Metrics interface { //nolint:interfacebloat // Only a little bit bloated. :|
	// Hit records a cache hit.
	Hit(name string)

	// Miss records a cache miss.
	Miss(name string)

	// Error records a cache error.
	Error(name string)

	// Write records a cache write.
	Write(name string)

	// Delete records a cache delete, i.e. due to garbage collection.
	Delete(name string)

	// WroteBytes records bytes written to the cache.
	WroteBytes(name string, b int)

	// DeletedBytes records bytes deleted from the cache.
	DeletedBytes(name string, b int)

	// ReadDuration records the time taken by a cache hit.
	ReadDuration(name string, d time.Duration)

	// WriteDuration records the time taken to write to cache.
	WriteDuration(name string, d time.Duration)
}

// A FunctionRunner runs a composition function.
type FunctionRunner interface {
	// RunFunction runs the named composition function.
	RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error)
}

// A FileBackedRunner wraps another function runner. It caches responses
// returned by that runner to the filesystem. It only caches responses that
// specify a TTL. Requests are served from cache if there's a cached response
// for an identical request with an unexpired TTL.
type FileBackedRunner struct {
	wrapped FunctionRunner
	fs      afero.Afero
	maxTTL  time.Duration
	log     logging.Logger
	metrics Metrics
}

// A FileBackedRunnerOption configures a FileBackedRunner.
type FileBackedRunnerOption func(r *FileBackedRunner)

// WithLogger specifies which logger the FileBackedRunner should use.
func WithLogger(l logging.Logger) FileBackedRunnerOption {
	return func(r *FileBackedRunner) {
		r.log = l
	}
}

// WithMetrics specifies which metrics the FileBackedRunner should use.
func WithMetrics(m Metrics) FileBackedRunnerOption {
	return func(r *FileBackedRunner) {
		r.metrics = m
	}
}

// WithMaxTTL clamps the maximum TTL for cached responses. The maximum TTL will
// be used to set the cache deadline for any response with a TTL greater than
// the max.
func WithMaxTTL(ttl time.Duration) FileBackedRunnerOption {
	return func(r *FileBackedRunner) {
		r.maxTTL = ttl
	}
}

// WithFilesystem specifies which filesystem implementation the FileBackedRunner
// should use. The runner will ignore its path argument and cache files at the
// root of this filesystem. Wrap your desired filesystem with afero.BasePathFS
// to cache files under a specific path.
func WithFilesystem(fs afero.Fs) FileBackedRunnerOption {
	return func(r *FileBackedRunner) {
		r.fs = afero.Afero{Fs: fs}
	}
}

// NewFileBackedRunner creates a new function runner that wraps another runner,
// caching its responses to the filesystem.
func NewFileBackedRunner(wrap FunctionRunner, path string, o ...FileBackedRunnerOption) *FileBackedRunner {
	r := &FileBackedRunner{
		wrapped: wrap,
		fs:      afero.Afero{Fs: afero.NewBasePathFs(afero.NewOsFs(), path)},
		log:     logging.NewNopLogger(),
		metrics: &NopMetrics{},
	}

	for _, fn := range o {
		fn(r)
	}

	return r
}

// RunFunction tries to return a response from cache. It falls back to calling
// the wrapped runner.
func (r *FileBackedRunner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	start := time.Now()
	log := r.log.WithValues("name", name)

	// If we don't have a cache key we can't perform a cache lookup, or
	// cache the response. Just send it on. This should never happen.
	if req.GetMeta().GetTag() == "" {
		log.Debug("RunFunctionResponse cache miss", "reason", ReasonEmptyRequestTag)
		r.metrics.Miss(name)

		return r.wrapped.RunFunction(ctx, name, req)
	}

	key := filepath.Join(name, req.GetMeta().GetTag())
	log = log.WithValues("cache-key", key)

	b, err := r.fs.ReadFile(key)
	if errors.Is(err, fs.ErrNotExist) {
		log.Debug("RunFunctionResponse cache miss", "reason", ReasonNotCached)
		r.metrics.Miss(name)

		return r.CacheFunction(ctx, name, req)
	}

	if err != nil {
		log.Info("RunFunctionResponse cache miss", "reason", ReasonError, "err", err)
		r.metrics.Miss(name)
		r.metrics.Error(name)

		return r.CacheFunction(ctx, name, req)
	}

	crsp := &v1alpha1.CachedRunFunctionResponse{}
	if err := proto.Unmarshal(b, crsp); err != nil {
		log.Info("RunFunctionResponse cache miss", "reason", ReasonError, "err", err)
		r.metrics.Miss(name)
		r.metrics.Error(name)

		return r.CacheFunction(ctx, name, req)
	}

	deadline := crsp.GetDeadline().AsTime()

	// Cached response has expired. This also covers the case where the
	// deadline isn't set - e.g. because we unmarshaled an empty file.
	if time.Now().After(deadline) {
		log.Debug("RunFunctionResponse cache miss", "reason", ReasonDeadlineExpired, "deadline", deadline)
		r.metrics.Miss(name)

		return r.CacheFunction(ctx, name, req)
	}

	log.Debug("RunFunctionResponse cache hit")
	r.metrics.Hit(name)
	r.metrics.ReadDuration(name, time.Since(start))

	return crsp.GetResponse(), nil
}

// CacheFunction runs a function and caches its response if the TTL is non-zero.
func (r *FileBackedRunner) CacheFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	// If we don't have a cache key we can't cache the response. Just send
	// it on. This should never happen.
	if req.GetMeta().GetTag() == "" {
		return r.wrapped.RunFunction(ctx, name, req)
	}

	key := filepath.Join(name, req.GetMeta().GetTag())
	log := r.log.WithValues("name", name, "cache-key", key)

	rsp, err := r.wrapped.RunFunction(ctx, name, req)
	if err != nil {
		return rsp, err
	}

	// Start timing the cache write _after_ we make the request.
	start := time.Now()

	ttl := rsp.GetMeta().GetTtl().AsDuration()
	if ttl == 0 {
		return rsp, nil
	}

	// Don't cache responses that have unfulfilled required resources.
	// Functions that request extra resources need to be called again once
	// those resources are available. If we cache the response before the
	// resources are provided, subsequent calls will get the cached response
	// without the function ever processing the extra resources.
	if hasUnfulfilledRequirements(req, rsp) {
		log.Debug("RunFunctionResponse has unfulfilled requirements, not caching")
		return rsp, nil
	}

	// Clamp the TTL to the allowed maximum, if set.
	if r.maxTTL > 0 && ttl > r.maxTTL {
		log.Debug("RunFunctionResponse cache clamped response TTL", "requested-ttl", ttl, "clamped-ttl", r.maxTTL)
		ttl = r.maxTTL
	}

	// Not all filesystems have btime, and Go doesn't expose a simple
	// interface to get it. So instead of adding TTL to btime at cache read
	// time, we instead compute a deadline at write time and wrap the cached
	// response.
	deadline := time.Now().Add(ttl)

	msg, err := proto.Marshal(&v1alpha1.CachedRunFunctionResponse{Deadline: timestamppb.New(deadline), Response: rsp})
	if err != nil {
		log.Info("RunFunctionResponse cache write error", "err", err)
		r.metrics.Error(name)

		return rsp, nil
	}

	if err := r.fs.MkdirAll(name, 0o700); err != nil {
		log.Info("RunFunctionResponse cache write error", "err", err)
		r.metrics.Error(name)

		return rsp, nil
	}

	// Write and rename a temp file to make our write 'atomic'. This ensure
	// we won't overwrite a cache file that we're currently reading.
	tmp, err := r.fs.TempFile(name, "")
	if err != nil {
		log.Info("RunFunctionResponse cache write error", "err", err)
		r.metrics.Error(name)

		return rsp, nil
	}

	if _, err := tmp.Write(msg); err != nil {
		_ = tmp.Close()

		log.Info("RunFunctionResponse cache write error", "err", err)
		r.metrics.Error(name)

		return rsp, nil
	}

	if err := tmp.Close(); err != nil {
		log.Info("RunFunctionResponse cache write error", "err", err)
		r.metrics.Error(name)

		return rsp, nil
	}

	if err := r.fs.Rename(tmp.Name(), key); err != nil {
		log.Info("RunFunctionResponse cache write error", "err", err)
		r.metrics.Error(name)

		return rsp, nil
	}

	log.Debug("RunFunctionResponse cache write", "deadline", deadline, "bytes", len(msg))
	r.metrics.Write(name)
	r.metrics.WriteDuration(name, time.Since(start))
	r.metrics.WroteBytes(name, len(msg))

	return rsp, nil
}

// GarbageCollectFiles runs every interval until the supplied context is
// cancelled. It garbage collects cached responses with expired deadlines.
func (r *FileBackedRunner) GarbageCollectFiles(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Debug("Stopping RunFunctionResponse cache garbage collector", "error", ctx.Err())
			return
		case <-t.C:
			if _, err := r.GarbageCollectFilesNow(ctx); err != nil {
				r.log.Info("Cannot garbage collect cached RunFunctionResponses", "error", err)
			}
		}
	}
}

// GarbageCollectFilesNow immediately garbage collects any cached responses with
// expired deadlines.
func (r *FileBackedRunner) GarbageCollectFilesNow(ctx context.Context) (int, error) {
	collected := 0
	iofs := afero.NewIOFS(r.fs)
	err := fs.WalkDir(iofs, "/", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Stop walking if our context is cancelled.
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if d.IsDir() {
			// Don't try to garbage collect the root of the cache.
			if path == "/" {
				return nil
			}

			entries, err := r.fs.ReadDir(path)
			if err != nil {
				r.log.Info("RunFunctionResponse cache error", "path", path, "error", err)
				return nil
			}

			if len(entries) > 0 {
				return nil
			}

			// Cleanup empty directories. We don't count these as a
			// garbage collected cache entry.
			if err := r.fs.Remove(path); err != nil {
				r.log.Info("RunFunctionResponse cache error", "path", path, "error", err)
			}

			return nil
		}

		// The cache layout is like /cache/function-name/request-hash,
		// so the directory name is our function name.
		name := filepath.Base(filepath.Dir(path))
		log := r.log.WithValues("name", name, "cache-key", path)

		b, err := r.fs.ReadFile(path)
		if err != nil {
			log.Info("RunFunctionResponse cache error", "error", err)
			r.metrics.Error(name)

			return nil
		}

		crsp := &v1alpha1.CachedRunFunctionResponse{}
		if err := proto.Unmarshal(b, crsp); err != nil {
			log.Info("RunFunctionResponse cache error", "error", err)
			r.metrics.Error(name)

			return nil
		}

		deadline := crsp.GetDeadline().AsTime()

		// Cached response is still valid.
		if time.Now().Before(deadline) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			log.Info("RunFunctionResponse cache error", "error", err)
			r.metrics.Error(name)
		}

		// There's a race here. It's possible CacheFunction will write a
		// new cache entry with a deadline in the future between where
		// we read the file and here where we remove it. We're okay with
		// this - it'll just mean we don't cache one response.
		//
		// There's no race with reading files. The file content won't
		// actually be deleted until ReadFile closes the fd.
		if err := r.fs.Remove(path); err != nil {
			log.Info("RunFunctionResponse cache error", "error", err)
			r.metrics.Error(name)

			return nil
		}

		collected++

		log.Debug("RunFunctionResponse cache delete", "deadline", deadline, "bytes", info.Size())
		r.metrics.Delete(name)
		r.metrics.DeletedBytes(name, int(info.Size()))

		return nil
	})

	return collected, err
}

// hasUnfulfilledRequirements returns true if the response contains requirements
// that are not yet fulfilled in the request. This indicates that the function
// needs to be called again once the requirements are satisfied.
func hasUnfulfilledRequirements(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) bool {
	requirements := rsp.GetRequirements()
	if requirements == nil || (len(requirements.GetResources()) == 0 && len(requirements.GetExtraResources()) == 0) { //nolint:staticcheck // Supporting deprecated field for backward compatibility
		return false
	}

	// Check if all requested resources are present in the request.
	// Support both old (extra_resources) and new (resources) field names.
	for name := range requirements.GetExtraResources() { //nolint:staticcheck // Supporting deprecated field for backward compatibility
		if req.GetExtraResources() == nil { //nolint:staticcheck // Supporting deprecated field for backward compatibility
			return true
		}
		if _, ok := req.GetExtraResources()[name]; !ok { //nolint:staticcheck // Supporting deprecated field for backward compatibility
			return true
		}
	}

	for name := range requirements.GetResources() {
		if req.GetRequiredResources() == nil {
			return true
		}
		if _, ok := req.GetRequiredResources()[name]; !ok {
			return true
		}
	}

	return false
}
