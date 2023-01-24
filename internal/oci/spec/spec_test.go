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

package spec

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	runtime "github.com/opencontainers/runtime-spec/specs-go"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type TestBundle struct{ path string }

func (b TestBundle) Path() string   { return b.path }
func (b TestBundle) Cleanup() error { return os.RemoveAll(b.path) }

func TestNew(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		o []Option
	}
	type want struct {
		s   *runtime.Spec
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"InvalidOption": {
			reason: "We should return an error if the supplied option is invalid.",
			args: args{
				o: []Option{func(s *runtime.Spec) error { return errBoom }},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplySpecOption),
			},
		},
		"Minimal": {
			reason: "It should be possible to apply an option to a new spec.",
			args: args{
				o: []Option{func(s *runtime.Spec) error {
					s.Annotations = map[string]string{"cool": "very"}
					return nil
				}},
			},
			want: want{
				s: func() *runtime.Spec {
					s, _ := New()
					s.Annotations = map[string]string{"cool": "very"}
					return s
				}(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := New(tc.args.o...)
			if diff := cmp.Diff(tc.want.s, got); diff != "" {
				t.Errorf("\n%s\nCreate(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCreate(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithCPULimit(t *testing.T) {
	var shares uint64 = 512
	var quota int64 = 50000

	type args struct {
		limit string
	}
	type want struct {
		s   *runtime.Spec
		err error
	}

	cases := map[string]struct {
		reason string
		s      *runtime.Spec
		args   args
		want   want
	}{
		"ParseLimitError": {
			reason: "We should return any error encountered while parsing the CPU limit.",
			s:      &runtime.Spec{},
			args: args{
				limit: "",
			},
			want: want{
				s:   &runtime.Spec{},
				err: errors.Wrap(resource.ErrFormatWrong, errParseCPULimit),
			},
		},
		"SuccessMilliCPUs": {
			reason: "We should set shares and quota according to the supplied milliCPUs.",
			s:      &runtime.Spec{},
			args: args{
				limit: "500m",
			},
			want: want{
				s: &runtime.Spec{
					Linux: &runtime.Linux{
						Resources: &runtime.LinuxResources{
							CPU: &runtime.LinuxCPU{
								Shares: &shares,
								Quota:  &quota,
							},
						},
					},
				},
			},
		},
		"SuccessCores": {
			reason: "We should set shares and quota according to the supplied cores.",
			s:      &runtime.Spec{},
			args: args{
				limit: "0.5",
			},
			want: want{
				s: &runtime.Spec{
					Linux: &runtime.Linux{
						Resources: &runtime.LinuxResources{
							CPU: &runtime.LinuxCPU{
								Shares: &shares,
								Quota:  &quota,
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := WithCPULimit(tc.args.limit)(tc.s)

			if diff := cmp.Diff(tc.want.s, tc.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nWithCPULimit(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWithCPULimit(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithMemoryLimit(t *testing.T) {
	var limit int64 = 512 * 1024 * 1024

	type args struct {
		limit string
	}
	type want struct {
		s   *runtime.Spec
		err error
	}

	cases := map[string]struct {
		reason string
		s      *runtime.Spec
		args   args
		want   want
	}{
		"ParseLimitError": {
			reason: "We should return any error encountered while parsing the memory limit.",
			s:      &runtime.Spec{},
			args: args{
				limit: "",
			},
			want: want{
				s:   &runtime.Spec{},
				err: errors.Wrap(resource.ErrFormatWrong, errParseMemoryLimit),
			},
		},
		"Success": {
			reason: "We should set the supplied memory limit.",
			s:      &runtime.Spec{},
			args: args{
				limit: "512Mi",
			},
			want: want{
				s: &runtime.Spec{
					Linux: &runtime.Linux{
						Resources: &runtime.LinuxResources{
							Memory: &runtime.LinuxMemory{
								Limit: &limit,
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := WithMemoryLimit(tc.args.limit)(tc.s)

			if diff := cmp.Diff(tc.want.s, tc.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nWithMemoryLimit(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWithMemoryLimit(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithHostNetwork(t *testing.T) {
	type want struct {
		s   *runtime.Spec
		err error
	}

	cases := map[string]struct {
		reason string
		s      *runtime.Spec
		want   want
	}{
		"RemoveNetworkNamespace": {
			reason: "We should remote the network namespace if it exists.",
			s: &runtime.Spec{
				Linux: &runtime.Linux{
					Namespaces: []runtime.LinuxNamespace{
						{Type: runtime.CgroupNamespace},
						{Type: runtime.NetworkNamespace},
					},
				},
			},
			want: want{
				s: &runtime.Spec{
					Mounts: []runtime.Mount{{
						Type:        "bind",
						Destination: "/etc/resolv.conf",
						Source:      "/etc/resolv.conf",
						Options:     []string{"rbind", "ro"},
					}},
					Linux: &runtime.Linux{
						Namespaces: []runtime.LinuxNamespace{
							{Type: runtime.CgroupNamespace},
						},
					},
				},
			},
		},
		"EmptySpec": {
			reason: "We should handle an empty spec without issue.",
			s:      &runtime.Spec{},
			want: want{
				s: &runtime.Spec{
					Mounts: []runtime.Mount{{
						Type:        "bind",
						Destination: "/etc/resolv.conf",
						Source:      "/etc/resolv.conf",
						Options:     []string{"rbind", "ro"},
					}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := WithHostNetwork()(tc.s)

			if diff := cmp.Diff(tc.want.s, tc.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nWithHostNetwork(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWithHostNetwork(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithImageConfig(t *testing.T) {
	type args struct {
		cfg    *ociv1.ConfigFile
		passwd string
		group  string
	}
	type want struct {
		s   *runtime.Spec
		err error
	}

	cases := map[string]struct {
		reason string
		s      *runtime.Spec
		args   args
		want   want
	}{
		"NoCommand": {
			reason: "We should return an error if the supplied image config has no entrypoint and no cmd.",
			s:      &runtime.Spec{},
			args: args{
				cfg: &ociv1.ConfigFile{},
			},
			want: want{
				s:   &runtime.Spec{},
				err: errors.New(errNoCmd),
			},
		},
		"UnresolvableUser": {
			reason: "We should return an error if there is no passwd data and a string username.",
			s:      &runtime.Spec{},
			args: args{
				cfg: &ociv1.ConfigFile{
					Config: ociv1.Config{
						Entrypoint: []string{"/bin/sh"},
						User:       "negz",
					},
				},
			},
			want: want{
				s: &runtime.Spec{
					Process: &runtime.Process{
						Args: []string{"/bin/sh"},
					},
				},
				err: errors.Wrap(errors.Errorf(errFmtNonExistentUser, "negz"), errResolveUser),
			},
		},
		"Success": {
			reason: "We should build a runtime config from the supplied image config.",
			s:      &runtime.Spec{},
			args: args{
				cfg: &ociv1.ConfigFile{
					Config: ociv1.Config{
						Hostname:   "coolhost",
						Entrypoint: []string{"/bin/sh"},
						Cmd:        []string{"cool"},
						Env:        []string{"COOL=very"},
						WorkingDir: "/",
						User:       "1000:100",
					},
				},
			},
			want: want{
				s: &runtime.Spec{
					Process: &runtime.Process{
						Args: []string{"/bin/sh", "cool"},
						Env:  []string{"COOL=very"},
						Cwd:  "/",
						User: runtime.User{
							UID: 1000,
							GID: 100,
						},
					},
					Hostname: "coolhost",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := WithImageConfig(tc.args.cfg, tc.args.passwd, tc.args.group)(tc.s)

			if diff := cmp.Diff(tc.want.s, tc.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nWithImageConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWithImageConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestParsePasswd(t *testing.T) {
	passwd := `
# Ensure that comments and leading whitespace are supported.
root:x:0:0:System administrator:/root:/run/current-system/sw/bin/zsh
negz:x:1000:100::/home/negz:/run/current-system/sw/bin/zsh
primary:x:1001:100::/home/primary:/run/current-system/sw/bin/zsh
`

	group := `
root:x:0:
wheel:x:1:negz
# This is primary's primary group, and doesnotexist doesn't exist in passwd.
users:x:100:primary,doesnotexist
`

	type args struct {
		passwd io.Reader
		group  io.Reader
	}
	type want struct {
		p   Passwd
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyFiles": {
			reason: "We should return an empty Passwd when both files are empty.",
			args: args{
				passwd: strings.NewReader(""),
				group:  strings.NewReader(""),
			},
			want: want{
				p: Passwd{},
			},
		},
		// TODO(negz): Should we try fuzz this?
		"MalformedPasswd": {
			reason: "We should return an error when the passwd file is malformed.",
			args: args{
				passwd: strings.NewReader("@!#!:f"),
				group:  strings.NewReader(""),
			},
			want: want{
				err: errors.Wrap(errors.New("record on line 1: wrong number of fields"), errParsePasswd),
			},
		},
		"MalformedGroup": {
			reason: "We should return an error when the group file is malformed.",
			args: args{
				passwd: strings.NewReader(""),
				group:  strings.NewReader("@!#!:f"),
			},
			want: want{
				err: errors.Wrap(errors.New("record on line 1: wrong number of fields"), errParseGroup),
			},
		},
		"NonIntegerPasswdUID": {
			reason: "We should return an error when the passwd file contains a non-integer uid.",
			args: args{
				passwd: strings.NewReader("username:password:uid:gid:gecos:homedir:shell"),
				group:  strings.NewReader(""),
			},
			want: want{
				err: errors.Wrap(errors.New("strconv.ParseUint: parsing \"uid\": invalid syntax"), errNonIntegerUID),
			},
		},
		"NonIntegerPasswdGID": {
			reason: "We should return an error when the passwd file contains a non-integer gid.",
			args: args{
				passwd: strings.NewReader("username:password:42:gid:gecos:homedir:shell"),
				group:  strings.NewReader(""),
			},
			want: want{
				err: errors.Wrap(errors.New("strconv.ParseUint: parsing \"gid\": invalid syntax"), errNonIntegerGID),
			},
		},
		"NonIntegerGroupGID": {
			reason: "We should return an error when the group file contains a non-integer gid.",
			args: args{
				passwd: strings.NewReader(""),
				group:  strings.NewReader("groupname:password:gid:username"),
			},
			want: want{
				err: errors.Wrap(errors.New("strconv.ParseUint: parsing \"gid\": invalid syntax"), errNonIntegerGID),
			},
		},
		"Success": {
			reason: "We should successfully parse well formatted passwd and group files.",
			args: args{
				passwd: strings.NewReader(passwd),
				group:  strings.NewReader(group),
			},
			want: want{
				p: Passwd{
					UID: map[Username]UID{
						"root":    0,
						"negz":    1000,
						"primary": 1001,
					},
					GID: map[Groupname]GID{
						"root":  0,
						"wheel": 1,
						"users": 100,
					},
					Groups: map[UID]Groups{
						0:    {PrimaryGID: 0},
						1000: {PrimaryGID: 100, AdditionalGIDs: []uint32{1}},
						1001: {PrimaryGID: 100},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParsePasswd(tc.args.passwd, tc.args.group)

			if diff := cmp.Diff(tc.want.p, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nParsePasswd(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParsePasswd(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestParsePasswdFiles(t *testing.T) {
	passwd := `
# Ensure that comments and leading whitespace are supported.
root:x:0:0:System administrator:/root:/run/current-system/sw/bin/zsh
negz:x:1000:100::/home/negz:/run/current-system/sw/bin/zsh
primary:x:1001:100::/home/primary:/run/current-system/sw/bin/zsh
`

	group := `
root:x:0:
wheel:x:1:negz
# This is primary's primary group, and doesnotexist doesn't exist in passwd.
users:x:100:primary,doesnotexist
`

	tmp, err := os.MkdirTemp(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer os.RemoveAll(tmp)

	_ = os.WriteFile(filepath.Join(tmp, "passwd"), []byte(passwd), 0600)
	_ = os.WriteFile(filepath.Join(tmp, "group"), []byte(group), 0600)

	type args struct {
		passwd string
		group  string
	}
	type want struct {
		p   Passwd
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoPasswdFile": {
			reason: "We should not return an error if the passwd file doesn't exist.",
			args: args{
				passwd: filepath.Join(tmp, "nonexist"),
				group:  filepath.Join(tmp, "group"),
			},
			want: want{
				p: Passwd{},
			},
		},
		"NoGroupFile": {
			reason: "We should not return an error if the group file doesn't exist.",
			args: args{
				passwd: filepath.Join(tmp, "passwd"),
				group:  filepath.Join(tmp, "nonexist"),
			},
			want: want{
				p: Passwd{},
			},
		},
		"Success": {
			reason: "We should successfully parse well formatted passwd and group files.",
			args: args{
				passwd: filepath.Join(tmp, "passwd"),
				group:  filepath.Join(tmp, "group"),
			},
			want: want{
				p: Passwd{
					UID: map[Username]UID{
						"root":    0,
						"negz":    1000,
						"primary": 1001,
					},
					GID: map[Groupname]GID{
						"root":  0,
						"wheel": 1,
						"users": 100,
					},
					Groups: map[UID]Groups{
						0:    {PrimaryGID: 0},
						1000: {PrimaryGID: 100, AdditionalGIDs: []uint32{1}},
						1001: {PrimaryGID: 100},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParsePasswdFiles(tc.args.passwd, tc.args.group)

			if diff := cmp.Diff(tc.want.p, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nParsePasswd(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParsePasswd(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithUser(t *testing.T) {
	type args struct {
		user string
		p    Passwd
	}
	type want struct {
		s   *runtime.Spec
		err error
	}

	// NOTE(negz): We 'test through' here only to test that WithUser can
	// distinguish a user (only) from a user and group and route them to the
	// right place; see TestWithUserOnly and TestWithUserAndGroup.
	cases := map[string]struct {
		reason string
		s      *runtime.Spec
		args   args
		want   want
	}{
		"TooManyColons": {
			reason: "We should return an error if the supplied user string contains more than one colon separator.",
			s:      &runtime.Spec{},
			args: args{
				user: "user:group:wat",
			},
			want: want{
				s:   &runtime.Spec{Process: &runtime.Process{}},
				err: errors.Errorf(errFmtTooManyColons, "user:group:wat"),
			},
		},
		"UIDOnly": {
			reason: "We should handle a user string that is a UID without error.",
			s:      &runtime.Spec{},
			args: args{
				user: "1000",
			},
			want: want{
				s: &runtime.Spec{Process: &runtime.Process{
					User: runtime.User{
						UID: 1000,
					},
				}},
			},
		},
		"UIDAndGID": {
			reason: "We should handle a user string that is a UID and GID without error.",
			s:      &runtime.Spec{},
			args: args{
				user: "1000:100",
			},
			want: want{
				s: &runtime.Spec{Process: &runtime.Process{
					User: runtime.User{
						UID: 1000,
						GID: 100,
					},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := WithUser(tc.args.user, tc.args.p)(tc.s)

			if diff := cmp.Diff(tc.want.s, tc.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nWithUser(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWithUser(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithUserOnly(t *testing.T) {
	type args struct {
		user string
		p    Passwd
	}
	type want struct {
		s   *runtime.Spec
		err error
	}

	cases := map[string]struct {
		reason string
		s      *runtime.Spec
		args   args
		want   want
	}{
		"UIDOnly": {
			reason: "We should handle a user string that is a UID without error.",
			s:      &runtime.Spec{},
			args: args{
				user: "1000",
			},
			want: want{
				s: &runtime.Spec{Process: &runtime.Process{
					User: runtime.User{
						UID: 1000,
					},
				}},
			},
		},
		"ResolveUIDGroups": {
			reason: "We should 'resolve' a UID's groups per the supplied Passwd data.",
			s:      &runtime.Spec{},
			args: args{
				user: "1000",
				p: Passwd{
					Groups: map[UID]Groups{
						1000: {
							PrimaryGID:     100,
							AdditionalGIDs: []uint32{1},
						},
					},
				},
			},
			want: want{
				s: &runtime.Spec{Process: &runtime.Process{
					User: runtime.User{
						UID:            1000,
						GID:            100,
						AdditionalGids: []uint32{1},
					},
				}},
			},
		},
		"NonExistentUser": {
			reason: "We should return an error if the supplied username doesn't exist in the supplied Passwd data.",
			s:      &runtime.Spec{},
			args: args{
				user: "doesnotexist",
				p:    Passwd{},
			},
			want: want{
				s:   &runtime.Spec{Process: &runtime.Process{}},
				err: errors.Errorf(errFmtNonExistentUser, "doesnotexist"),
			},
		},
		"ResolveUserToUID": {
			reason: "We should 'resolve' a username to a UID per the supplied Passwd data.",
			s:      &runtime.Spec{},
			args: args{
				user: "negz",
				p: Passwd{
					UID: map[Username]UID{
						"negz": 1000,
					},
				},
			},
			want: want{
				s: &runtime.Spec{Process: &runtime.Process{
					User: runtime.User{
						UID: 1000,
					},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := WithUserOnly(tc.args.user, tc.args.p)(tc.s)

			if diff := cmp.Diff(tc.want.s, tc.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nWithUserOnly(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWithUserOnly(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithUserAndGroup(t *testing.T) {
	type args struct {
		user  string
		group string
		p     Passwd
	}
	type want struct {
		s   *runtime.Spec
		err error
	}

	cases := map[string]struct {
		reason string
		s      *runtime.Spec
		args   args
		want   want
	}{
		"UIDAndGID": {
			reason: "We should handle a UID and GID without error.",
			s:      &runtime.Spec{},
			args: args{
				user:  "1000",
				group: "100",
			},
			want: want{
				s: &runtime.Spec{Process: &runtime.Process{
					User: runtime.User{
						UID: 1000,
						GID: 100,
					},
				}},
			},
		},
		"ResolveAdditionalGIDs": {
			reason: "We should resolve any additional GIDs in the supplied Passwd data.",
			s:      &runtime.Spec{},
			args: args{
				user:  "1000",
				group: "100",
				p: Passwd{
					Groups: map[UID]Groups{
						1000: {
							PrimaryGID:     42, // This should be ignored, since an explicit GID was supplied.
							AdditionalGIDs: []uint32{1},
						},
					},
				},
			},
			want: want{
				s: &runtime.Spec{Process: &runtime.Process{
					User: runtime.User{
						UID:            1000,
						GID:            100,
						AdditionalGids: []uint32{1},
					},
				}},
			},
		},
		"NonExistentUser": {
			reason: "We should return an error if the supplied username doesn't exist in the supplied Passwd data.",
			s:      &runtime.Spec{},
			args: args{
				user: "doesnotexist",
				p:    Passwd{},
			},
			want: want{
				s:   &runtime.Spec{Process: &runtime.Process{}},
				err: errors.Errorf(errFmtNonExistentUser, "doesnotexist"),
			},
		},
		"NonExistentGroup": {
			reason: "We should return an error if the supplied group doesn't exist in the supplied Passwd data.",
			s:      &runtime.Spec{},
			args: args{
				user:  "exists",
				group: "doesnotexist",
				p: Passwd{
					UID: map[Username]UID{"exists": 1000},
				},
			},
			want: want{
				s:   &runtime.Spec{Process: &runtime.Process{}},
				err: errors.Errorf(errFmtNonExistentGroup, "doesnotexist"),
			},
		},
		"ResolveUserAndGroupToUIDAndGID": {
			reason: "We should 'resolve' a username to a UID and a groupname to a GID per the supplied Passwd data.",
			s:      &runtime.Spec{},
			args: args{
				user:  "negz",
				group: "users",
				p: Passwd{
					UID: map[Username]UID{
						"negz": 1000,
					},
					GID: map[Groupname]GID{
						"users": 100,
					},
				},
			},
			want: want{
				s: &runtime.Spec{Process: &runtime.Process{
					User: runtime.User{
						UID: 1000,
						GID: 100,
					},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := WithUserAndGroup(tc.args.user, tc.args.group, tc.args.p)(tc.s)

			if diff := cmp.Diff(tc.want.s, tc.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nWithUserAndGroup(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWithUserAndGroup(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
