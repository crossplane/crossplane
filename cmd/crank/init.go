package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// initCmd initializes a repository from a template
type initCmd struct {
	Function initFunctionCmd `cmd:"" help:"Initialize a Function from the repo template for a selected language."`
}

type initFunctionCmd struct {
	Name string `arg:"" help:"Name of the Function to initialize."`

	Language  functionLanguage `short:"l" help:"Language of the Function to initialize." enum:"go" default:"go"`
	Directory string           `short:"d" help:"Path of the directory to initialize." default:"." type:"path"`
}

type functionLanguage string

const (
	functionLanguageGo functionLanguage = "go"
)

func (c *initFunctionCmd) GetURLForLanguage() (string, error) {
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

	fs := osfs.New(c.Directory, osfs.WithBoundOS())

	url, err := c.GetURLForLanguage()
	if err != nil {
		return errors.Wrapf(err, "failed to get URL for language %s", c.Language)
	}

	r, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL:   url,
		Depth: 1,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to clone repo")
	}

	ref, err := r.Head()
	if err != nil {
		return errors.Wrapf(err, "failed to get repository's HEAD")
	}

	// TODO(phisco): replace placeholders for the name all around the
	// 	repository? Maybe we can just agree on some markdown text in the
	// 	repos to print to let the user know what to do next?

	_, err = fmt.Fprintf(k.Stdout, "Initialized Function %q in directory %q from %s/tree/%s (%s)\n", c.Name, c.Directory, url, ref.Hash().String(), ref.Name().Short())
	return err
}
