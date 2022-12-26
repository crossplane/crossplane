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

// Package spec implements OCI runtime spec support.
package spec

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	runtime "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane/internal/oci/store"
)

const (
	errApplySpecOption  = "cannot apply OCI runtime spec option"
	errCreateFile       = "cannot create file"
	errWriteFile        = "cannot write file"
	errCloseFile        = "cannot write file"
	errNoCmd            = "function OCI image must specify entrypoint and/or cmd"
	errParsePasswd      = "cannot parse passwd file data"
	errParseGroup       = "cannot parse group file data"
	errResolveUser      = "cannot resolve user specified by OCI image config"
	errNonIntegerUID    = "cannot parse non-integer UID"
	errNonIntegerGID    = "cannot parse non-integer GID"
	errOpenPasswdFile   = "cannot open passwd file"
	errOpenGroupFile    = "cannot open group file"
	errNewSpec          = "cannot create new OCI runtime spec"
	errWriteSpec        = "cannot write OCI runtime spec"
	errParsePasswdFiles = "cannot parse container's /etc/passwd and/or /etc/group files"

	errFmtTooManyColons    = "cannot parse user %q (too many colon separators)"
	errFmtNonExistentUser  = "cannot resolve UID of user %q that doesn't exist in container's /etc/passwd"
	errFmtNonExistentGroup = "cannot resolve GID of group %q that doesn't exist in container's /etc/group"
)

// An Option specifies optional OCI runtime configuration.
type Option func(s *runtime.Spec) error

// WithImageConfig extends a Spec with configuration derived from an OCI image
// config file. If the image config specifies a user it will be resolved against
// the supplied Passwd data.
func WithImageConfig(cfg *ociv1.ConfigFile, p Passwd) Option {
	// TODO(negz): Break these up into smaller options? e.g. FooFromImageConfig?
	return func(s *runtime.Spec) error {
		if cfg.Config.Hostname != "" {
			s.Hostname = cfg.Config.Hostname
		}

		if s.Process == nil {
			s.Process = &runtime.Process{}
		}

		s.Process.Env = append(s.Process.Env, cfg.Config.Env...)

		args := make([]string, 0, len(cfg.Config.Entrypoint)+len(cfg.Config.Cmd))
		args = append(args, cfg.Config.Entrypoint...)
		args = append(args, cfg.Config.Cmd...)
		if len(args) == 0 {
			return errors.New(errNoCmd)
		}

		s.Process.Args = args

		if cfg.Config.WorkingDir != "" {
			s.Process.Cwd = cfg.Config.WorkingDir
		}

		if cfg.Config.User != "" {
			if err := WithUser(cfg.Config.User, p)(s); err != nil {
				return errors.Wrap(err, errResolveUser)
			}
		}

		return nil
	}
}

// A Username within an /etc/passwd file.
type Username string

// A Groupname within an /etc/group file.
type Groupname string

// A UID within an /etc/passwd file.
type UID int

// A GID within an /etc/passwd or /etc/group file.
type GID int

// Unknown UID and GIDs.
const (
	UnknownUID = UID(-1)
	UnknownGID = GID(-1)
)

// Passwd (and group) file data.
type Passwd struct {
	UID    map[Username]UID
	GID    map[Groupname]GID
	Groups map[UID]Groups
}

// Groups represents a user's groups.
type Groups struct {
	// Elsewhere we use types like UID and GID for self-documenting map keys. We
	// use uint32 here for convenience. It's what runtime.User wants and we
	// don't want to have to convert a slice of GID to a slice of uint32.

	PrimaryGID     uint32
	AdditionalGIDs []uint32
}

// ParsePasswdFiles parses the passwd and group files at the supplied paths.
func ParsePasswdFiles(passwd, group string) (Passwd, error) {
	p, err := os.Open(passwd) //nolint:gosec // We intentionally take a variable here.
	if err != nil {
		return Passwd{}, errors.Wrap(err, errOpenPasswdFile)
	}
	defer p.Close() //nolint:errcheck,gosec // Only open for reading.

	g, err := os.Open(group) //nolint:gosec // We intentionally take a variable here.
	if err != nil {
		return Passwd{}, errors.Wrap(err, errOpenGroupFile)
	}
	defer g.Close() //nolint:errcheck,gosec // Only open for reading.

	return ParsePasswd(p, g)
}

// ParsePasswd parses the supplied passwd and group data.
func ParsePasswd(passwd, group io.Reader) (Passwd, error) { //nolint:gocyclo // Breaking each loop into its own function seems more complicated.
	out := Passwd{
		UID:    make(map[Username]UID),
		GID:    make(map[Groupname]GID),
		Groups: make(map[UID]Groups),
	}

	// Formatted as name:password:UID:GID:GECOS:directory:shell
	p := csv.NewReader(passwd)
	p.Comma = ':'
	p.Comment = '#'
	p.TrimLeadingSpace = true
	p.FieldsPerRecord = 7 // len(r) will be guaranteed to be 7.

	for {
		r, err := p.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Passwd{}, errors.Wrap(err, errParsePasswd)
		}

		username := r[0]
		uid, err := strconv.Atoi(r[2])
		if err != nil {
			return Passwd{}, errors.Wrap(err, errNonIntegerUID)
		}
		gid, err := strconv.Atoi(r[3])
		if err != nil {
			return Passwd{}, errors.Wrap(err, errNonIntegerGID)
		}

		out.UID[Username(username)] = UID(uid)
		out.Groups[UID(uid)] = Groups{PrimaryGID: uint32(gid)}
	}

	// Formatted as group_name:password:GID:comma_separated_user_list
	g := csv.NewReader(group)
	g.Comma = ':'
	g.Comment = '#'
	g.TrimLeadingSpace = true
	g.FieldsPerRecord = 4 // len(r) will be guaranteed to be 4.

	for {
		r, err := g.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Passwd{}, errors.Wrap(err, errParseGroup)
		}

		groupname := r[0]
		gid, err := strconv.Atoi(r[2])
		if err != nil {
			return Passwd{}, errors.Wrap(err, errNonIntegerGID)
		}

		out.GID[Groupname(groupname)] = GID(gid)

		users := r[3]

		// This group has no users (except those with membership via passwd).
		if users == "" {
			continue
		}

		for _, u := range strings.Split(users, ",") {
			uid, ok := out.UID[Username(u)]
			if !ok || gid == int(out.Groups[uid].PrimaryGID) {
				// Either this user doesn't exist, or they do and the group is
				// their primary group. Either way we want to skip it.
				continue
			}
			g := out.Groups[uid]
			g.AdditionalGIDs = append(g.AdditionalGIDs, uint32(gid))
			out.Groups[uid] = g
		}
	}

	return out, nil
}

// WithUser resolves an OCI image config user string in order to set the spec's
// process user. According to the OCI image config v1.0 spec: "For Linux based
// systems, all of the following are valid: user, uid, user:group, uid:gid,
// uid:group, user:gid. If group/GID is not specified, the default group and
// supplementary groups of the given user/UID in /etc/passwd from the container
// are applied."
func WithUser(user string, p Passwd) Option {
	return func(s *runtime.Spec) error {
		if s.Process == nil {
			s.Process = &runtime.Process{}
		}

		parts := strings.Split(user, ":")
		switch len(parts) {
		case 1:
			return WithUserOnly(parts[0], p)(s)
		case 2:
			return WithUserAndGroup(parts[0], parts[1], p)(s)
		default:
			return errors.Errorf(errFmtTooManyColons, user)
		}
	}
}

// WithUserOnly resolves an OCI Image config user string in order to set the
// spec's process user. The supplied user string must either be an integer UID
// (that may or may not exist in the container's /etc/passwd) or a username that
// exists in the container's /etc/passwd. The supplied user string must not
// contain any group information.
func WithUserOnly(user string, p Passwd) Option {
	return func(s *runtime.Spec) error {
		if s.Process == nil {
			s.Process = &runtime.Process{}
		}

		uid := UnknownUID

		// If user is an integer we treat it as a UID.
		if v, err := strconv.Atoi(user); err == nil {
			uid = UID(v)
		}

		// If user is not an integer we must resolve it to one using data
		// extracted from the container's passwd file.
		if uid == UnknownUID {
			v, ok := p.UID[Username(user)]
			if !ok {
				return errors.Errorf(errFmtNonExistentUser, user)
			}
			uid = v
		}

		// At this point the UID was either explicitly specified or
		// resolved. Note that if the UID doesn't exist in the supplied
		// passwd and group data we'll set its GID to 0. This behaviour isn't
		// specified by the OCI spec, but matches what containerd does.
		s.Process.User = runtime.User{
			UID:            uint32(uid),
			GID:            p.Groups[uid].PrimaryGID,
			AdditionalGids: p.Groups[uid].AdditionalGIDs,
		}
		return nil
	}
}

// WithUserAndGroup resolves an OCI image config user string in order to set the
// spec's process user. The supplied user string must either be an integer UID
// (that may or may not exist in the container's /etc/passwd) or a username that
// exists in the container's /etc/passwd. The supplied group must either be an
// integer GID (that may or may not exist in the container's /etc/group) or a
// group name that exists in the container's /etc/group.
func WithUserAndGroup(user, group string, p Passwd) Option {
	return func(s *runtime.Spec) error {
		if s.Process == nil {
			s.Process = &runtime.Process{}
		}

		uid, gid := UnknownUID, UnknownGID

		// If user and/or group are integers we treat them as UID/GIDs.
		if v, err := strconv.Atoi(user); err == nil {
			uid = UID(v)
		}
		if v, err := strconv.Atoi(group); err == nil {
			gid = GID(v)
		}

		// If user and/or group weren't integers we must resolve them to a
		// UID/GID that exists within the container's passwd/group files.
		if uid == UnknownUID {
			v, ok := p.UID[Username(user)]
			if !ok {
				return errors.Errorf(errFmtNonExistentUser, user)
			}
			uid = v
		}
		if gid == UnknownGID {
			v, ok := p.GID[Groupname(group)]
			if !ok {
				return errors.Errorf(errFmtNonExistentGroup, group)
			}
			gid = v
		}

		// At this point the UID and GID were either explicitly specified or
		// resolved. All we need to do is supply any additional GIDs.
		s.Process.User = runtime.User{
			UID:            uint32(uid),
			GID:            uint32(gid),
			AdditionalGids: p.Groups[uid].AdditionalGIDs,
		}
		return nil
	}
}

// WithHostNetwork configures the container to share the host's (i.e. xfn
// container's) network namespace.
func WithHostNetwork() Option {
	return func(s *runtime.Spec) error {
		s.Mounts = append(s.Mounts, runtime.Mount{
			Type:        "bind",
			Destination: "/etc/resolv.conf",
			Source:      "/etc/resolv.conf",
			Options:     []string{"rbind", "ro"},
		})
		if s.Linux == nil {
			s.Linux = &runtime.Linux{}
		}
		s.Linux.Namespaces = append(s.Linux.Namespaces, runtime.LinuxNamespace{Type: runtime.NetworkNamespace})
		return nil
	}
}

// New produces a new OCI runtime spec (i.e. config.json).
func New(o ...Option) (*runtime.Spec, error) {
	// NOTE(negz): Most of this is what `crun spec --rootless` produces.
	spec := &runtime.Spec{
		Version: runtime.Version,
		Process: &runtime.Process{
			User: runtime.User{UID: 0, GID: 0},
			Env:  []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
			Cwd:  "/",
			Capabilities: &runtime.LinuxCapabilities{
				Bounding: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Effective: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Permitted: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Ambient: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
			},
			Rlimits: []runtime.POSIXRlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: 1024,
					Soft: 1024,
				},
			},
		},
		Root: &runtime.Root{
			Path:     store.DirRootFS,
			Readonly: true, // TODO(negz): Make this configurable?
		},
		Hostname: "xfn",
		Mounts: []runtime.Mount{
			{
				Type:        "bind",
				Destination: "/proc",
				Source:      "/proc",
				Options:     []string{"nosuid", "noexec", "nodev", "rbind"},
			},
			{
				Type:        "tmpfs",
				Destination: "/dev",
				Source:      "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
			},
			{
				Type:        "sysfs",
				Destination: "/sys",
				Source:      "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
			},

			{
				Destination: "/dev/pts",
				Type:        "devpts",
				Source:      "devpts",
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
			},
			{
				Destination: "/dev/mqueue",
				Type:        "mqueue",
				Source:      "mqueue",
				Options:     []string{"nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/sys/fs/cgroup",
				Type:        "cgroup",
				Source:      "cgroup",
				Options:     []string{"rprivate", "nosuid", "noexec", "nodev", "relatime", "ro"},
			},
		},
		// TODO(negz): Do we need a seccomp policy? Our host probably has one.
		Linux: &runtime.Linux{
			Resources: &runtime.LinuxResources{
				Devices: []runtime.LinuxDeviceCgroup{
					{
						Allow:  false,
						Access: "rwm",
					},
				},
				Pids: &runtime.LinuxPids{
					Limit: 32768,
				},
			},
			Namespaces: []runtime.LinuxNamespace{
				{Type: runtime.PIDNamespace},
				{Type: runtime.IPCNamespace},
				{Type: runtime.UTSNamespace},
				{Type: runtime.MountNamespace},
				{Type: runtime.CgroupNamespace},
				{Type: runtime.NetworkNamespace},
			},
			MaskedPaths: []string{
				"/proc/acpi",
				"/proc/kcore",
				"/proc/keys",
				"/proc/latency_stats",
				"/proc/timer_list",
				"/proc/timer_stats",
				"/proc/sched_debug",
				"/proc/scsi",
				"/sys/firmware",
				"/sys/fs/selinux",
				"/sys/dev/block",
			},
			ReadonlyPaths: []string{
				"/proc/asound",
				"/proc/bus",
				"/proc/fs",
				"/proc/irq",
				"/proc/sys",
				"/proc/sysrq-trigger",
			},
		},
	}

	for _, fn := range o {
		if err := fn(spec); err != nil {
			return nil, errors.Wrap(err, errApplySpecOption)
		}
	}

	return spec, nil
}

// TODO(negz): This should take user-supplied config too (i.e. from a gRPC call)

// Create the OCI runtime spec for the supplied bundle. This spec is derived
// from the supplied OCI image config file, potentially supplemented with data
// located within the bundle's rootfs (such as the /etc/passwd file).
func Create(b store.Bundle, cfg *ociv1.ConfigFile) error {
	rootfs := store.RootFSPath(b)
	p, err := ParsePasswdFiles(filepath.Join(rootfs, "etc", "passwd"), filepath.Join(rootfs, "etc", "group"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.Wrap(err, errParsePasswdFiles)
	}

	s, err := New(WithImageConfig(cfg, p))
	if err != nil {
		return errors.Wrap(err, errNewSpec)
	}

	return errors.Wrap(Write(store.SpecPath(b), s), errWriteSpec)
}

// Write the supplied OCI runtime spec to a file at the supplied path.
func Write(path string, cfg *runtime.Spec) error {
	rcf, err := os.Create(path) //nolint:gosec // Creating a path supplied as a variable is intentional.
	if err != nil {
		return errors.Wrap(err, errCreateFile)
	}

	if err := json.NewEncoder(rcf).Encode(cfg); err != nil {
		_ = rcf.Close()
		return errors.Wrap(err, errWriteFile)
	}

	return errors.Wrap(rcf.Close(), errCloseFile)
}
