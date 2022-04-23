//go:build unix

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
	"os/exec"
	"syscall"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// NOTE(negz): We build this function for unix so that folks running (e.g.)
// Darwin can build and test the code, even though it's only really useful for
// Linux systems.

// StdioPipes creates and returns pipes that will be connected to the supplied
// command's stdio when it starts. It calls fchown(2) to ensure all pipes are
// owned by the supplied user and group ID; this ensures that the command can
// read and write its stdio even when xfn is running as root (in the parent
// namespace) and the command is not.
func StdioPipes(cmd *exec.Cmd, uid, gid int) (*Stdio, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, errCreateStdinPipe)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, errCreateStdoutPipe)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, errors.Wrap(err, errCreateStderrPipe)
	}

	// StdinPipe and friends above return "our end" of the pipe - i.e. stdin is
	// the io.WriteCloser we can use to write to the command's stdin. They also
	// setup the "command's end" of the pipe - i.e. cmd.Stdin is the io.Reader
	// the command can use to read its stdin. In all cases these pipes _should_
	// be *os.Files.
	for _, s := range []any{stdin, stdout, stderr, cmd.Stdin, cmd.Stdout, cmd.Stderr} {
		f, ok := s.(interface{ Fd() uintptr })
		if !ok {
			return nil, errors.Errorf("stdio pipe (type: %T) missing required Fd() method", f)
		}
		// Fchown does not take an integer fd on Windows.
		if err := syscall.Fchown(int(f.Fd()), uid, gid); err != nil {
			return nil, errors.Wrap(err, errChownFd)
		}
	}

	return &Stdio{Stdin: stdin, Stdout: stdout, Stderr: stderr}, nil
}
