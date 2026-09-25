// This is a self-contained Go module for a tiny Crossplane composition function
// used only by Crossplane's e2e tests. It is intentionally a nested module so
// the parent repo's `go build ./...` does not try to compile it. The e2e tests
// depend only on the published function image, not on this source.
//
// The pinned versions below are a starting point; run `go mod tidy` before
// building to resolve a consistent set (see README.md).
module github.com/crossplane/crossplane/test/e2e/functions/revision-id

go 1.24.0

require (
	github.com/alecthomas/kong v1.6.0
	github.com/crossplane/function-sdk-go v0.5.0
	k8s.io/apimachinery v0.32.1
)
