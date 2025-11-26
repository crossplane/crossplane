/*
Copyright 2020 The Crossplane Authors.

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

// Package plugin implements plugin discovery and execution for the crossplane CLI.
package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// PluginPrefix is the prefix for plugin binaries
	PluginPrefix = "crossplane-"
)

// FindPlugin searches for a plugin binary in PATH
func FindPlugin(name string) (string, error) {
	pluginName := PluginPrefix + name
	if runtime.GOOS == "windows" {
		pluginName += ".exe"
	}

	// Search in PATH
	path, err := exec.LookPath(pluginName)
	if err != nil {
		return "", fmt.Errorf("unable to find plugin %q in PATH", pluginName)
	}

	return path, nil
}

// Execute runs a plugin with the given arguments
func Execute(pluginPath string, args []string) error {
	cmd := exec.Command(pluginPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}

// ListPlugins returns a list of available plugins found in PATH
func ListPlugins() ([]string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil, nil
	}

	var plugins []string
	seen := make(map[string]bool)

	pathDirs := filepath.SplitList(pathEnv)
	for _, dir := range pathDirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue // Skip directories we can't read
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			name := file.Name()
			if !strings.HasPrefix(name, PluginPrefix) {
				continue
			}

			// Remove prefix and .exe suffix (if on Windows)
			pluginName := strings.TrimPrefix(name, PluginPrefix)
			pluginName = strings.TrimSuffix(pluginName, ".exe")

			// Check if executable
			fullPath := filepath.Join(dir, name)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			// On Unix-like systems, check if executable
			if runtime.GOOS != "windows" {
				if info.Mode()&0111 == 0 {
					continue
				}
			}

			// Avoid duplicates
			if !seen[pluginName] {
				plugins = append(plugins, pluginName)
				seen[pluginName] = true
			}
		}
	}

	return plugins, nil
}
