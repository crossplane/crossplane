//go:build linux

/*
Copyright 2022 The Crossplane Authors.

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

package xfn

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"google.golang.org/protobuf/proto"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
)

// NOTE(negz): Technically _all_ of the containerized Composition Functions
// implementation is only useful on Linux, but we want to support building what
// we can on other operating systems (e.g. Darwin) to make it possible for folks
// running them to ensure that code compiles and passes tests during
// development. Avoid adding code to this file unless it actually needs Linux to
// run.

// Error strings.
const (
	errCreateStdioPipes  = "cannot create stdio pipes"
	errStartSpark        = "cannot start " + spark
	errCloseStdin        = "cannot close stdin pipe"
	errReadStdout        = "cannot read from stdout pipe"
	errReadStderr        = "cannot read from stderr pipe"
	errMarshalRequest    = "cannot marshal RunFunctionRequest for " + spark
	errWriteRequest      = "cannot write RunFunctionRequest to " + spark + " stdin"
	errUnmarshalResponse = "cannot unmarshal RunFunctionRequest from " + spark + " stdout"
)

// How many UIDs and GIDs to map from the parent to the child user namespace, if
// possible. Doing so requires CAP_SETUID and CAP_SETGID.
const (
	UserNamespaceUIDs = 65536
	UserNamespaceGIDs = 65536
	MaxStdioBytes     = 100 << 20 // 100 MB
)

// The subcommand of xfn to invoke - i.e. "xfn spark <source> <bundle>"
const spark = "spark"

// HasCapSetUID returns true if this process has CAP_SETUID.
func HasCapSetUID() bool {
	pc := cap.GetProc()
	setuid, _ := pc.GetFlag(cap.Effective, cap.SETUID)
	return setuid
}

// HasCapSetGID returns true if this process has CAP_SETGID.
func HasCapSetGID() bool {
	pc := cap.GetProc()
	setgid, _ := pc.GetFlag(cap.Effective, cap.SETGID)
	return setgid
}

// RunFunction runs a function as a rootless OCI container. Functions that
// return non-zero, or that cannot be executed in the first place (e.g. because
// they cannot be fetched from the registry) will return an error.
func (r *ContainerRunner) RunFunction(ctx context.Context, req *v1alpha1.RunFunctionRequest) (*v1alpha1.RunFunctionResponse, error) {
	r.log.Debug("Running function", "image", req.Image)

	/*
		We want to create an overlayfs with the cached rootfs as the lower layer
		and the bundle's rootfs as the upper layer, if possible. Kernel 5.11 and
		later supports using overlayfs inside a user (and mount) namespace. The
		best way to run code in a user namespace in Go is to execute a separate
		binary; the unix.Unshare syscall affects only one OS thread, and the Go
		scheduler might move the goroutine to another.

		Therefore we execute a shim - xfn spark - in a new user and mount
		namespace. spark fetches and caches the image, creates an OCI runtime
		bundle, then then executes an OCI runtime in order to actually execute
		the function.
	*/
	cmd := exec.CommandContext(ctx, os.Args[0], spark, "--cache-dir="+r.cache, fmt.Sprintf("--max-stdio-bytes=%d", MaxStdioBytes)) //nolint:gosec // We're intentionally executing with variable input.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: r.rootUID, Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: r.rootGID, Size: 1}},
	}

	// When we have CAP_SETUID and CAP_SETGID (i.e. typically when root), we can
	// map a range of UIDs (0 to 65,336) inside the user namespace to a range in
	// its parent. We can also drop privileges (in the parent user namespace) by
	// running spark as root in the user namespace.
	if r.setuid {
		cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: r.rootUID, Size: UserNamespaceUIDs}}
		cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: r.rootGID, Size: UserNamespaceGIDs}}
		cmd.SysProcAttr.GidMappingsEnableSetgroups = true

		/*
			UID and GID 0 here are relative to the new user namespace - i.e. they
			correspond to HostID in the parent. We're able to do this because
			Go's exec.Command will:

			1. Call clone(2) to create a child process in a new user namespace.
			2. In the child process, wait for /proc/self/uid_map to be written.
			3. In the parent process, write the child's /proc/$pid/uid_map.
			4. In the child process, call setuid(2) and setgid(2) per Credential.
			5. In the child process, call execve(2) to execute spark.

			Per user_namespaces(7) the child process created by clone(2) starts
			out with a complete set of capabilities in the new user namespace
			until the call to execve(2) causes them to be recalculated. This
			includes the CAP_SETUID and CAP_SETGID necessary to become UID 0 in
			the child user namespace, effectively dropping privileges to UID
			100000 in the parent user namespace.

			https://github.com/golang/go/blob/1b03568/src/syscall/exec_linux.go#L446
		*/
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: 0, Gid: 0}
	}

	stdio, err := StdioPipes(cmd, r.rootUID, r.rootGID)
	if err != nil {
		return nil, errors.Wrap(err, errCreateStdioPipes)
	}

	b, err := proto.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, errMarshalRequest)
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, errStartSpark)
	}
	if _, err := stdio.Stdin.Write(b); err != nil {
		return nil, errors.Wrap(err, errWriteRequest)
	}

	// Closing the write end of the stdio pipe will cause the read end to return
	// EOF. This is necessary to avoid a function blocking forever while reading
	// from stdin.
	if err := stdio.Stdin.Close(); err != nil {
		return nil, errors.Wrap(err, errCloseStdin)
	}

	// We must read all of stdout and stderr before calling cmd.Wait, which
	// closes the underlying pipes.
	// Limited to MaxStdioBytes to avoid OOMing if the function writes a lot of
	// data to stdout or stderr.
	stdout, err := io.ReadAll(io.LimitReader(stdio.Stdout, MaxStdioBytes))
	if err != nil {
		return nil, errors.Wrap(err, errReadStdout)
	}

	stderr, err := io.ReadAll(io.LimitReader(stdio.Stderr, MaxStdioBytes))
	if err != nil {
		return nil, errors.Wrap(err, errReadStderr)
	}

	if err := cmd.Wait(); err != nil {
		// TODO(negz): Handle stderr being too long to be a useful error.
		return nil, errors.Errorf("%w: %s", err, bytes.TrimSuffix(stderr, []byte("\n")))
	}

	rsp := &v1alpha1.RunFunctionResponse{}
	return rsp, errors.Wrap(proto.Unmarshal(stdout, rsp), errUnmarshalResponse)
}
