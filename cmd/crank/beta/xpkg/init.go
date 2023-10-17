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

package xpkg

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// initCmd initializes a repository from a template
type initCmd struct {
	Function initFunctionCmd `cmd:"" help:"Initialize a Function from the repo template for a selected language."`
}

type initFunctionCmd struct {
	Language functionLanguage `arg:"" help:"Language of the Function to initialize." enum:"go"`
	Name     string           `arg:"" help:"Name of the Function to initialize."`

	CustomRepo string `short:"r" help:"URL of the custom template repository to use instead of the default one for the language."`
	Directory  string `short:"d" help:"Path of the directory to initialize." default:"." type:"path"`
}

type functionLanguage string

const (
	functionLanguageGo functionLanguage = "go"
)

func (c *initFunctionCmd) GetTemplateURL() (string, error) {
	if c.CustomRepo != "" {
		return c.CustomRepo, nil
	}
	switch c.Language {
	case functionLanguageGo:
		return "https://github.com/crossplane/function-template-go", nil
	default:
		return "", errors.Errorf("unknown language %s", c.Language)
	}
}

func (c *initFunctionCmd) Run(k *kong.Context, logger logging.Logger) error {
	f, err := os.Stat(c.Directory)
	switch {
	case err == nil && !f.IsDir():
		return errors.Errorf("path %s is not a directory", c.Directory)
	case os.IsNotExist(err):
		if err := os.MkdirAll(c.Directory, 0750); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", c.Directory)
		}
		logger.Debug("Created directory", "path", c.Directory)
	case err != nil:
		return errors.Wrapf(err, "failed to stat directory %s", c.Directory)
	}

	// check the directory only contains allowed files/directories, error out otherwise
	if err := c.checkDirectoryContent(); err != nil {
		return err
	}

	fs := osfs.New(c.Directory, osfs.WithBoundOS())

	repoURL, err := c.GetTemplateURL()
	if err != nil {
		return errors.Wrapf(err, "failed to get URL for language %s", c.Language)
	}

	r, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL:   repoURL,
		Depth: 1,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to clone repo from %q", repoURL)
	}

	ref, err := r.Head()
	if err != nil {
		return errors.Wrapf(err, "failed to get repository's HEAD from %q", repoURL)
	}

	// TODO(phisco): replace placeholders for the name all around the
	// 	repository? Maybe we can just agree on some markdown text in the
	// 	repos to print to let the user know what to do next?

	_, err = fmt.Fprintf(k.Stdout, "Initialized Function %q in directory %q from %s (%s)\n", c.Name, c.Directory, getPrettyURL(logger, repoURL, ref), ref.Name().Short())
	return err
}

func getPrettyURL(logger logging.Logger, repoURL string, ref *plumbing.Reference) string {
	prettyURL, err := url.JoinPath(repoURL, "tree", ref.Hash().String())
	if err != nil {
		// we won't show the commit URL in this case, no big issue
		logger.Debug("Failed to create commit URL, will just use original url", "error", err)
		return repoURL
	}
	return prettyURL
}

func (c *initFunctionCmd) checkDirectoryContent() error {
	entries, err := os.ReadDir(c.Directory)
	if err != nil {
		return errors.Wrapf(err, "failed to read directory %s", c.Directory)
	}
	notAllowedEntries := make([]string, 0)
	for _, entry := range entries {
		// .git directory is allowed
		if entry.Name() == ".git" && entry.IsDir() {
			continue
		}
		// add all other entries to the list of unauthorized entries
		notAllowedEntries = append(notAllowedEntries, entry.Name())
	}
	if len(notAllowedEntries) > 0 {
		return errors.Errorf("directory %s is not empty, contains existing files/directories: %s", c.Directory, strings.Join(notAllowedEntries, ", "))
	}
	return nil
}
