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
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	runtime "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// This program's name!
const programName = "ignition"

// Error strings.
const (
	errOpenConfigFile   = "cannot open OCI config file"
	errDecodeConfigFile = "cannot decode OCI config file"
	errCloseConfigFile  = "cannot close OCI config file"
	errRootMissing      = "OCI config file is missing root.path"
	errRootAbsolute     = "OCI config file root.path must be relative to the root of the bundle"
	errMkdir            = "cannot make directory"
	errRuntime          = "cannot invoke OCI runtime"
	errCopySource       = "cannot copy source rootfs"
	errOpenDst          = "cannot open destination file"
	errOpenSrc          = "cannot open source file"
	errCopy             = "cannot copy file"
	errCloseDst         = "cannot close destination file"
	errCloseSrc         = "cannot close source file"
	errChownDst         = "cannot change owner of destination file"
)

// Bundle paths.
const (
	config = "config.json"
)

type cli struct {
	Config  string `help:"OCI config file, relative to root of the bundle." default:"config.json"`
	Runtime string `help:"OCI runtime binary to invoke." default:"/usr/local/bin/crun"`
	State   string `help:"OCI runtime state (i.e. --root) directory." default:"/tmp/ignition"`

	Source string `arg:"" help:"Source of the bundle's rootfs. Will either be copied or used as a lower overlayfs filesystem." type:"existingdir"`
	Bundle string `arg:"" help:"Root of the bundle." type:"existingdir"`
}

func (c *cli) Run() error {
	f, err := os.Open(filepath.Join(c.Bundle, config))
	if err != nil {
		return errors.Wrap(err, errOpenConfigFile)
	}

	var cfg runtime.Spec
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		_ = f.Close()
		return errors.Wrap(err, errDecodeConfigFile)
	}

	if err := f.Close(); err != nil {
		return errors.Wrap(err, errCloseConfigFile)
	}

	if cfg.Root == nil {
		return errors.New(errRootMissing)
	}

	if filepath.IsAbs(cfg.Root.Path) {
		return errors.New(errRootAbsolute)
	}

	l := log.New(os.Stderr, programName+": ", 0)
	if err := overlay(c.Bundle, cfg.Root.Path, c.Source); err != nil {
		l.Printf("cannot create rootfs overlay - falling back to copying source: %v", err)
		if err := copy(c.Bundle, cfg.Root.Path, c.Source); err != nil {
			return errors.Wrap(err, errCopySource)
		}
	}

	if err := os.MkdirAll(c.State, 0750); err != nil {
		return errors.Wrap(err, errMkdir)
	}

	//nolint:gosec // Executing with user-supplied input is intentional.
	cmd := exec.Command(c.Runtime, "--root", c.State, "run", "--bundle", c.Bundle, uuid.NewString())

	// TODO(negz): Is this sufficient to plumb/forward these?
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	var rerr exec.ExitError
	if errors.As(err, &rerr) {
		_ = f.Close()
		os.Exit(rerr.ExitCode())
	}

	return errors.Wrap(err, errRuntime)
}

func copy(bundle, rootfs, src string) error {
	err := filepath.Walk(src, func(srcPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		dstPath := filepath.Join(bundle, rootfs, strings.TrimPrefix(srcPath, src))

		if info.IsDir() {
			return errors.Wrap(os.MkdirAll(dstPath, info.Mode()), errMkdir)
		}

		//nolint:gosec // Opening with user-supplied input is intentional.
		src, err := os.Open(srcPath)
		if err != nil {
			return errors.Wrap(err, errOpenSrc)
		}

		//nolint:gosec // Opening with user-supplied input is intentional
		dst, err := os.OpenFile(dstPath, os.O_CREATE, info.Mode())
		if err != nil {
			return errors.Wrap(err, errOpenDst)
		}

		if _, err := io.Copy(dst, src); err != nil {
			_ = src.Close()
			_ = dst.Close()
			return errors.Wrap(err, errCopy)
		}

		if err := src.Close(); err != nil {
			_ = dst.Close()
			return errors.Wrap(err, errCloseSrc)
		}
		if err := dst.Close(); err != nil {
			return errors.Wrap(err, errCloseDst)
		}

		if s, ok := info.Sys().(*syscall.Stat_t); ok {
			if err := os.Chown(dstPath, int(s.Uid), int(s.Gid)); err != nil {
				return errors.Wrap(err, errChownDst)
			}
		}

		return nil
	})
	return err
}

func main() {
	c := &cli{}
	ctx := kong.Parse(c,
		kong.Name(programName),
		kong.Description("Prepares an OCI bundle's rootfs, then invokes an OCI runtime."),
		kong.UsageOnError(),
	)
	ctx.FatalIfErrorf(c.Run())
}
