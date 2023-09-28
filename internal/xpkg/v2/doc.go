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

// Note(lsviben): This package is an upstream copy of the https://github.com/upbound/up/tree/main/internal/xpkg
// package, which is in turn a modified version of internal/xpkg. Due to the fact it diverged over time, we
// decided to copy it over as v2. Ideally, the as the old commands get deprecated in v1, we can merge the two.

// Package xpkg contains utilities for building and linting Crossplane packages.
package xpkg
