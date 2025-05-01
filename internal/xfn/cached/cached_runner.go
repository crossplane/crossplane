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
	"encoding/binary"
	"io"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
	"google.golang.org/protobuf/proto"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
)

// A CacheMissReason indicates what caused a cache miss.
type CacheMissReason string

// Cache miss reasons.
const (
	ReasonNotCached       CacheMissReason = "NotCached"
	ReasonDeadlineExpired CacheMissReason = "DeadlineExpired"
	ReasonEmptyRequestTag CacheMissReason = "EmptyRequestTag"
	ReasonEmptyResponse   CacheMissReason = "EmptyResponse"
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

	f, err := r.fs.Open(key)
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
	defer f.Close() //nolint:errcheck // Only open for reading. Nothing useful to do with the error.

	// The first 8 bytes of our file is always our deadline.
	header := make([]byte, 8)
	if _, err := f.Read(header); err != nil {
		log.Info("RunFunctionResponse cache miss", "reason", ReasonError, "err", err)
		r.metrics.Miss(name)
		r.metrics.Error(name)
		return r.CacheFunction(ctx, name, req)
	}

	deadline := time.Unix(int64(binary.LittleEndian.Uint64(header)), 0) //nolint:gosec // False positive for G115.

	// Cached response has expired.
	if time.Now().After(deadline) {
		log.Debug("RunFunctionResponse cache miss", "reason", ReasonDeadlineExpired, "deadline", deadline)
		r.metrics.Miss(name)
		return r.CacheFunction(ctx, name, req)
	}

	// Everything after the first 8 bytes is our cached response.
	msg, err := io.ReadAll(f)
	if err != nil {
		log.Info("RunFunctionResponse cache miss", "reason", ReasonError, "err", err)
		r.metrics.Miss(name)
		r.metrics.Error(name)
		return r.CacheFunction(ctx, name, req)
	}

	// Our cached file is just 8 bytes of deadline, with no response. This
	// is unlikely but if it did happen proto.Unmarshal would pass without
	// error and we'd return an empty response.
	if len(msg) == 0 {
		log.Debug("RunFunctionResponse cache miss", "reason", ReasonEmptyResponse)
		r.metrics.Miss(name)
		return r.CacheFunction(ctx, name, req)
	}

	rsp := &fnv1.RunFunctionResponse{}
	if err := proto.Unmarshal(msg, rsp); err != nil {
		log.Info("RunFunctionResponse cache miss", "reason", ReasonError, "err", err)
		r.metrics.Miss(name)
		r.metrics.Error(name)
		return r.CacheFunction(ctx, name, req)
	}

	log.Debug("RunFunctionResponse cache hit")
	r.metrics.Hit(name)
	r.metrics.ReadDuration(name, time.Since(start))
	return rsp, nil
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

	// Not all filesystems have btime, and Go doesn't expose a simple
	// interface to get it. We just write our own at the beginning of the
	// file. We use basic binary encoding (as opposed to gob or protobuf)
	// because the fixed length makes it easy to slice the file into the
	// timestamp header and the protobuf message body.
	deadline := time.Now().Add(ttl)
	header := make([]byte, 8)
	binary.LittleEndian.PutUint64(header, uint64(time.Now().Add(ttl).Unix())) //nolint:gosec // False positive for G115.

	msg, err := proto.Marshal(rsp)
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

	if err := r.fs.WriteFile(key, append(header, msg...), 0o600); err != nil { //nolint:makezero // We want deadline to be padded to 8 bytes.
		log.Info("RunFunctionResponse cache write error", "err", err)
		r.metrics.Error(name)
		return rsp, nil
	}

	log.Debug("RunFunctionResponse cache write", "deadline", deadline, "bytes", len(header)+len(msg))
	r.metrics.Write(name)
	r.metrics.WriteDuration(name, time.Since(start))
	r.metrics.WroteBytes(name, len(header)+len(msg))
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

		f, err := r.fs.Open(path)
		if err != nil {
			log.Info("RunFunctionResponse cache error", "error", err)
			r.metrics.Error(name)
			return nil
		}
		defer f.Close() //nolint:errcheck // Only open for reading.

		// The first 8 bytes of our file is always our deadline.
		header := make([]byte, 8)
		if _, err := f.Read(header); err != nil {
			log.Info("RunFunctionResponse cache error", "error", err)
			r.metrics.Error(name)
			return nil
		}

		deadline := time.Unix(int64(binary.LittleEndian.Uint64(header)), 0) //nolint:gosec // False positive for G115.

		// Cached response is still valid.
		if time.Now().Before(deadline) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			log.Info("RunFunctionResponse cache error", "error", err)
			r.metrics.Error(name)
		}

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
