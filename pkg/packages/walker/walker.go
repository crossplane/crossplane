/*
Copyright 2019 The Crossplane Authors.

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

package walker

import (
	"os"
	"path/filepath"
)

// A Step is a function that takes the name and bytes of a file and processes it in some way
type Step func(path string, bytes []byte) error

// ResourceWalker builds a Step list with the intent to Walk them
type ResourceWalker interface {
	Walk() error
	AddStep(pattern string, step Step)
}

// ReadFileWalker is used to walk a file tree and read the contents of each file
// This is used for mocking. `afero.Afero` fulfills this interface.
// `filepath.Walk` and `ioutil.ReadFile` are functions in the core packages that fit
// these signatures.
type ReadFileWalker interface {
	ReadFile(string) ([]byte, error)
	Walk(string, filepath.WalkFunc) error
}

// ResourceDir contains the the directory to walk on and the WalkFuncs to perform
type ResourceDir struct {
	// Base is the base (root) dir that will be walked. It is expected to be an absolute path,
	// i.e. have a root to the path, at the very least "/".
	Base    string
	Walker  ReadFileWalker
	walkers []filepath.WalkFunc
}

// composeWalkers composes several filepath.WalkFuncs into a single filepath.Walkfunc
func composeWalkers(walkers ...filepath.WalkFunc) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		for _, walker := range walkers {
			if err := walker(path, info, err); err != nil {
				return err
			}
		}
		return nil
	}
}

// Walk applies all of the Step functions against the Base directory
func (rd *ResourceDir) Walk() error {
	err := rd.Walker.Walk(rd.Base, composeWalkers(rd.walkers...))
	return err
}

// AddStep adds a Step to the Walker
// Each Step will be given the bytes and filepath of resource files matching the supplied name pattern
func (rd *ResourceDir) AddStep(pattern string, step Step) {
	wrappedStep := func(path string, info os.FileInfo, err error) error {
		if match, err := filepath.Match(pattern, info.Name()); err != nil {
			return err
		} else if match {
			b, err := rd.Walker.ReadFile(path)
			if err != nil {
				return err
			}
			return step(path, b)
		}
		return nil
	}

	rd.walkers = append(rd.walkers, wrappedStep)
}
