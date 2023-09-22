/*
Copyright 2023 The Crossplane Authors.

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

package input

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type mockFile struct {
	mockReadFn  func([]byte) (int, error)
	mockWriteFn func([]byte) (int, error)
	mockFd      func() uintptr
}

func (m *mockFile) Read(b []byte) (int, error) {
	return m.mockReadFn(b)
}

func (m *mockFile) Write(b []byte) (int, error) {
	return m.mockWriteFn(b)
}

func (m *mockFile) Fd() uintptr {
	return m.mockFd()
}

type mockTTY struct {
	mockIsTerminal   func(int) bool
	mockReadPassword func(int) ([]byte, error)
}

func (m *mockTTY) IsTerminal(fd int) bool {
	return m.mockIsTerminal(fd)
}

func (m *mockTTY) ReadPassword(fd int) ([]byte, error) {
	return m.mockReadPassword(fd)
}

func TestPrompt(t *testing.T) {
	errBoom := errors.New("boom")
	input := "hello\n"
	label := "Input"
	type args struct {
		label     string
		sensitive bool
	}
	cases := map[string]struct {
		reason   string
		prompter Prompter
		args     args
		want     string
		err      error
	}{
		"NotATTY": {
			reason: "Error should be returned if prompt is called and input is not TTY.",
			prompter: &defaultPrompter{
				in: &mockFile{
					mockFd: func() uintptr { return 10 },
				},
				tty: &mockTTY{
					mockIsTerminal: func(int) bool { return false },
				},
			},
			err: errors.New(errNotTTY),
		},
		"ErrWrite": {
			reason: "Error should be returned if we fail to write output.",
			prompter: &defaultPrompter{
				in: &mockFile{
					mockFd: func() uintptr { return 1 },
				},
				out: &mockFile{
					mockWriteFn: func([]byte) (int, error) {
						return 0, errBoom
					},
				},
				tty: &mockTTY{
					mockIsTerminal: func(int) bool { return true },
				},
			},
			err: errBoom,
		},
		"ErrNotSensitive": {
			reason: "Error should be returned if we fail to read non-sensitive input.",
			prompter: &defaultPrompter{
				in: &mockFile{
					mockFd: func() uintptr { return 1 },
					mockReadFn: func(b []byte) (int, error) {
						return 0, errBoom
					},
				},
				out: &mockFile{
					mockWriteFn: func([]byte) (int, error) {
						return 0, nil
					},
				},
				tty: &mockTTY{
					mockIsTerminal: func(int) bool { return true },
				},
			},
			err: errBoom,
		},
		"SuccessfulNotSensitive": {
			reason: "Should return result if successfully read non-sensitive input.",
			prompter: &defaultPrompter{
				in: &mockFile{
					mockFd: func() uintptr { return 1 },
					mockReadFn: func(b []byte) (int, error) {
						return copy(b, input), nil
					},
				},
				out: &mockFile{
					mockWriteFn: func([]byte) (int, error) {
						return 0, nil
					},
				},
				tty: &mockTTY{
					mockIsTerminal: func(int) bool { return true },
				},
			},
			want: "hello",
		},
		"ErrorSensitive": {
			reason: "Error should be returned if we fail to read sensitive input.",
			args: args{
				sensitive: true,
			},
			prompter: &defaultPrompter{
				in: &mockFile{
					mockFd: func() uintptr { return 1 },
				},
				out: &mockFile{
					mockWriteFn: func([]byte) (int, error) {
						return 0, nil
					},
				},
				tty: &mockTTY{
					mockIsTerminal: func(int) bool { return true },
					mockReadPassword: func(int) ([]byte, error) {
						return []byte{}, errBoom
					},
				},
			},
			err: errBoom,
		},
		"SuccessfulSensitive": {
			reason: "Should return result if successfully read sensitive input.",
			args: args{
				label:     label,
				sensitive: true,
			},
			prompter: &defaultPrompter{
				in: &mockFile{
					mockFd: func() uintptr { return 1 },
				},
				out: &mockFile{
					mockWriteFn: func(b []byte) (int, error) {
						return len(b), nil
					},
				},
				tty: &mockTTY{
					mockIsTerminal: func(int) bool { return true },
					mockReadPassword: func(int) ([]byte, error) {
						return []byte(input), nil
					},
				},
			},
			want: input,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := tc.prompter.Prompt(tc.args.label, tc.args.sensitive)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPrompt(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, p); diff != "" {
				t.Errorf("\n%s\nPrompt(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
