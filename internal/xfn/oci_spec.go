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
	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	runtime "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errNoCmd = "function OCI image must specify entrypoint and/or cmd"
)

const (
	// This is the UID and GID of the distroless nonroot user.
	// https://github.com/GoogleContainerTools/distroless/blob/449ded/base/base.bzl#L8
	// TODO(negz): Make this configurable?
	nonroot = 65532
)

// NewRuntimeConfig produces an OCI runtime spec (i.e. config.json) from the
// supplied OCI image config file.
func NewRuntimeConfig(cfg *ociv1.ConfigFile) (*runtime.Spec, error) {

	args := make([]string, 0, len(cfg.Config.Entrypoint)+len(cfg.Config.Cmd))
	args = append(args, cfg.Config.Entrypoint...)
	args = append(args, cfg.Config.Cmd...)
	if len(args) == 0 {
		return nil, errors.New(errNoCmd)
	}

	var umask uint32 = 18
	spec := &runtime.Spec{
		Version: runtime.Version,
		Process: &runtime.Process{
			User: runtime.User{
				UID:   nonroot,
				GID:   nonroot,
				Umask: &umask,
			},
			Args:         args,
			Env:          cfg.Config.Env,
			Cwd:          cfg.Config.WorkingDir,
			Capabilities: &runtime.LinuxCapabilities{
				// TODO(negz): Some of these.
			},
		},
		Root: &runtime.Root{
			Path: rootfs,
		},
		Hostname: cfg.Config.Hostname,
		// TODO(negz): Bind mount the host container's /etc/hosts and
		// /etc/resolv.conf if networking is enabled?
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
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
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
		// TODO(negz): Configure sane default cgroup limits and seccomp policy?
		Linux: &runtime.Linux{
			Namespaces: []runtime.LinuxNamespace{
				{Type: runtime.PIDNamespace},
				{Type: runtime.IPCNamespace},
				{Type: runtime.UTSNamespace},
				{Type: runtime.MountNamespace},
				{Type: runtime.CgroupNamespace},

				// TODO(negz): Remove this namespace to allow network access by
				// sharing the 'host' (container) network namespace.
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

	return spec, nil
}
