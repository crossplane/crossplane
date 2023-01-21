//go:build !linux

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

package overlay

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const errLinuxOnly = "overlayfs is only only supported on Linux"

// Mount returns an error on non-Linux systems.
func (m TmpFSMount) Mount() error { return errors.New(errLinuxOnly) }

// Unmount returns an error on non-Linux systems.
func (m TmpFSMount) Unmount() error { return errors.New(errLinuxOnly) }

// Mount returns an error on non-Linux systems.
func (m OverlayMount) Mount() error { return errors.New(errLinuxOnly) }

// Unmount returns an error on non-Linux systems.
func (m OverlayMount) Unmount() error { return errors.New(errLinuxOnly) }
