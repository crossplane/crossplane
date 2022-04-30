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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errMountTmpfs     = "cannot mount tmpfs for rootfs overlay upper and work dirs"
	errMountOverlayfs = "cannot mount rootfs overlay"
)

// Bundle paths.
const (
	tmpfs = "tmpfs"
	upper = "upper"
	work  = "work"
)

func overlay(bundle, rootfs, source string) error {
	if err := os.Mkdir(filepath.Join(bundle, tmpfs), 0750); err != nil {
		return errors.Wrap(err, errMkdir)
	}

	// Share all mounts with peer group members - i.e. child mount namespaces.
	var flags uintptr = unix.MS_SHARED | unix.MS_REC
	if err := unix.Mount("tmpfs", filepath.Join(bundle, tmpfs), "tmpfs", flags, ""); err != nil {
		return errors.Wrap(err, errMountTmpfs)
	}

	for _, dir := range []string{
		filepath.Join(bundle, tmpfs, upper),
		filepath.Join(bundle, tmpfs, work),
	} {
		if err := os.Mkdir(dir, 0750); err != nil {
			return errors.Wrap(err, errMkdir)
		}
	}

	data := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		source,
		filepath.Join(bundle, tmpfs, upper),
		filepath.Join(bundle, tmpfs, work),
	)
	return errors.Wrap(unix.Mount("overlay", filepath.Join(bundle, rootfs), "overlay", flags, data), errMountOverlayfs)
}
