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
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const (
	notes      = "NOTES.txt"
	initScript = "init.sh"
)

// WellKnownTemplates are short aliases for template repositories.
func WellKnownTemplates() map[string]string {
	return map[string]string{
		"provider-template":        "https://github.com/crossplane/provider-template",
		"provider-template-upjet":  "https://github.com/upbound/upjet-provider-template",
		"function-template-go":     "https://github.com/crossplane/function-template-go",
		"function-template-python": "https://github.com/crossplane/function-template-python",
	}
}

// initCmd initializes a new package repository from a template repository.
type initCmd struct {
	Name     string `arg:"" help:"The name of the new package to initialize."`
	Template string `arg:"" help:"The template name or URL to use to initialize the new package."`

	Directory     string `short:"d" default:"." type:"path" help:"The directory to initialize. It must be empty. It will be created if it doesn't exist."`
	RunInitScript bool   `short:"r" name:"run-init-script" help:"Runs the init.sh script if it exists without prompting"`
}

func (c *initCmd) Help() string {
	tpl := `
This command initializes a directory that you can use to build a package. It
uses a template to initialize the directory. It can use any Git repository as a
template.

You can specify either a full Git URL or a well-known name as a template. The
following well-known template names are supported:

%s

If the template contains NOTES.txt in its root directory, it will be
printed to stdout. This is useful for providing instructions for how
to use the template.

If the template contains init.sh in its root directory, it will be optionally
printed out and executed. This is useful for providing a script that can be
used to initialize the package automatically. Use the -r flag to run the
script without prompting.

Examples:

  # Initialize a new Go Composition Function named function-example.
  crossplane beta xpkg init function-example function-template-go

  # Initialize a new Provider named provider-example from a custom template.
  crossplane beta xpkg init provider-example https://github.com/crossplane/provider-template-custom

  # Initialize a new Go Composition Function named function-example and run
  # its init.sh script (if it exists) without prompting the user or displaying its contents.
  crossplane beta xpkg init function-example function-template-go --run-init-script
`

	b := strings.Builder{}
	for name, url := range WellKnownTemplates() {
		b.WriteString(fmt.Sprintf(" - %s (%s)\n", name, url))
	}

	return fmt.Sprintf(tpl, b.String())
}

func (c *initCmd) Run(k *kong.Context, logger logging.Logger) error { //nolint:gocyclo // file check switch and print error check make it over the top
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

	repoURL, ok := WellKnownTemplates()[c.Template]
	if !ok {
		// If the template isn't one of the well-known ones, assume its a URL.
		repoURL = c.Template
	}

	fs := osfs.New(c.Directory, osfs.WithBoundOS())
	r, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL:   repoURL,
		Depth: 1,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to clone repository from %q", repoURL)
	}

	ref, err := r.Head()
	if err != nil {
		return errors.Wrapf(err, "failed to get repository's HEAD from %q", repoURL)
	}

	if _, err := fmt.Fprintf(k.Stdout, "Initialized package %q in directory %q from %s (%s)\n",
		c.Name, c.Directory, getPrettyURL(logger, repoURL, ref), ref.Name().Short()); err != nil {
		return errors.Wrap(err, "failed to write to stdout")
	}

	if err := c.handleNotes(k.Stdout, logger); err != nil {
		return errors.Wrap(err, "failed to handle NOTES.txt")
	}

	if err := c.handleInitScript(k, logger); err != nil {
		return errors.Wrap(err, "failed to handle init.sh")
	}

	return nil
}

// handleNotes prints the NOTES.txt file in the template
// repository, if it exists.
func (c *initCmd) handleNotes(w io.Writer, logger logging.Logger) error {
	notesFile := filepath.Join(c.Directory, notes)
	f, err := os.Stat(notesFile)
	switch {
	case os.IsNotExist(err):
		// no NOTES.txt file, skip
		logger.Debug("No NOTES.txt found, skipping")
		return nil
	case err != nil:
		return errors.Wrapf(err, "failed to stat notes file %s", notesFile)
	case f.IsDir():
		return errors.Errorf("%s is not a file", notesFile)
	}

	return errors.Wrapf(printFile(w, notesFile), "failed to print file %s", notesFile)
}

// handleInitScript runs the init.sh script in the template repository, if it
// exists.
func (c *initCmd) handleInitScript(k *kong.Context, logger logging.Logger) error {
	scriptFile := filepath.Join(c.Directory, initScript)
	f, err := os.Stat(scriptFile)
	switch {
	case os.IsNotExist(err):
		// no init.sh file, skip
		logger.Debug("No init.sh found, skipping")
		return nil
	case err != nil:
		return errors.Wrapf(err, "failed to stat init.sh file %s", scriptFile)
	case f.IsDir():
		return errors.Errorf("%s is not a file", scriptFile)
	}

	if c.RunInitScript {
		return errors.Wrapf(runScript(k, scriptFile, c.Name, c.Directory), "failed to run init script %s", scriptFile)
	}

	if _, err := fmt.Fprintln(k.Stdout, "\nFound init.sh script!"); err != nil {
		return errors.Wrap(err, "failed to write to stdout")
	}

	return errors.Wrapf(initPrompt(k, scriptFile, c.Name, c.Directory), "failed to handle init script %s", scriptFile)
}

func initPrompt(k *kong.Context, scriptFile, name, dir string) error {
	answer, err := prompt(k, "Do you want to run it? [y]es/[n]o/[v]iew: ")
	if err != nil {
		return errors.Wrap(err, "failed to prompt user")
	}

	switch answer {
	case "y", "yes":
		return errors.Wrapf(runScript(k, scriptFile, name, dir), "failed to run init script %s", scriptFile)
	case "v", "view":
		if err := printFile(k.Stdout, scriptFile); err != nil {
			return errors.Wrapf(err, "failed to print file %s", scriptFile)
		}
		return initPrompt(k, scriptFile, name, dir)
	}
	return nil
}

func printFile(w io.Writer, path string) error {
	// read and print the script
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return errors.Wrapf(err, "failed to open file %s", path)
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return errors.Wrapf(err, "failed to read file %s", path)
	}
	if _, err := fmt.Fprintf(w, "\n%s\n", content); err != nil {
		return errors.Wrap(err, "failed to write to stdout")
	}
	return nil
}

func runScript(k *kong.Context, scriptFile string, args ...string) error {
	cmd := exec.Command(scriptFile, args...)
	cmd.Stdout = k.Stdout
	cmd.Stderr = k.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func prompt(k *kong.Context, question string) (string, error) {
	if _, err := fmt.Fprintf(k.Stdout, "%s", question); err != nil {
		return "", errors.Wrap(err, "failed to write to stdout")
	}
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return "", errors.Wrap(err, "failed to read from stdin")
	}
	return answer, nil
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

func (c *initCmd) checkDirectoryContent() error {
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
