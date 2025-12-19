/*
Copyright 2025 The Crossplane Authors.

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

package test

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gonvenience/ytbx"
	"github.com/homeport/dyff/pkg/dyff"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/cmd/crank/render"
)

const (
	// CompositeFileName is the name of the file containing the composite resource.
	CompositeFileName = "composite-resource.yaml"
	// ExtraResourcesFileName is the name of the file containing extra resources.
	ExtraResourcesFileName = "extra-resources.yaml"
	// ObservedResourcesFileName is the name of the file containing observed resources.
	ObservedResourcesFileName = "observed-resources.yaml"
	// FunctionsFileName is the name of the file containing the function configurations.
	FunctionsFileName = "dev-functions.yaml"
)

// Inputs contains all inputs to the test process.
type Inputs struct {
	TestDir              string
	FileSystem           afero.Fs
	OutputFile           string
	WriteExpectedOutputs bool // If true, write/update expected.yaml files instead of comparing
}

// Outputs contains test results.
type Outputs struct {
	TestDirs []string // Directories containing tests
	Pass     bool     // Test result
}

// Test renders composite resources and either compares them with expected outputs or writes new expected outputs.
func Test(ctx context.Context, log logging.Logger, in Inputs) (Outputs, error) {
	// Find all directories with a composite-resource.yaml file
	testDirs, err := findTestDirectories(in.FileSystem, in.TestDir)
	if err != nil {
		return Outputs{}, err
	}

	log.Debug("Test directory paths", "directories", testDirs)

	// Process tests sequentially
	results := make(map[string][]byte)
	for _, dir := range testDirs {
		output, err := renderTest(ctx, log, in.FileSystem, dir)
		if err != nil {
			return Outputs{}, errors.Wrapf(err, "failed to process %q", dir)
		}
		results[dir] = output
	}

	testFailed := false
	// Write expected outputs or compare (default is compare)
	if in.WriteExpectedOutputs {
		// Write the outputs to files
		for _, dir := range testDirs {
			actualOutput := results[dir]
			outputPath := filepath.Join(dir, in.OutputFile)
			if err := afero.WriteFile(in.FileSystem, outputPath, actualOutput, 0o644); err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot write output to %q", outputPath)
			}
			log.Debug("Wrote output", "path", outputPath)
		}
	} else {
		// Compare expected vs. actual (default behavior)
		log.Info("Comparing outputs with dyff")

		for _, dir := range testDirs {
			expectedOutput, err := afero.ReadFile(in.FileSystem, filepath.Join(dir, "expected.yaml"))
			if err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot read expected output for test %q", dir)
			}

			expectedDocs, err := ytbx.LoadDocuments(expectedOutput)
			if err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot parse expected YAML for %q", dir)
			}

			actualDocs, err := ytbx.LoadDocuments(results[dir])
			if err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot parse actual YAML for %q", dir)
			}

			report, err := dyff.CompareInputFiles(
				ytbx.InputFile{Documents: expectedDocs},
				ytbx.InputFile{Documents: actualDocs},
			)
			if err != nil {
				return Outputs{}, errors.Wrapf(err, "cannot compare files for %q", dir)
			}

			if len(report.Diffs) > 0 {
				testFailed = true
				log.Debug("Test failed", "directory", dir)
				_, _ = fmt.Fprintln(os.Stdout, "TEST FAILED", dir)

				reportWriter := &dyff.HumanReport{
					Report:     report,
					Indent:     2,
					OmitHeader: true,
				}

				var buf bytes.Buffer
				if err := reportWriter.WriteReport(&buf); err != nil {
					return Outputs{}, errors.Wrapf(err, "cannot write diff report for %q", dir)
				}

				// extra diff indent
				_, _ = fmt.Fprintln(os.Stdout, "  "+strings.ReplaceAll(buf.String(), "\n", "\n  "))
			} else {
				log.Debug("Test passed", "directory", dir)
				_, _ = fmt.Fprintln(os.Stdout, "TEST PASSED", dir)
			}
		}

		if testFailed {
			return Outputs{}, errors.New("test failed: differences found between expected and actual outputs")
		}

		log.Info("All tests passed")
	}

	return Outputs{
		TestDirs: testDirs,
		Pass:     !testFailed,
	}, nil
}

// findTestDirectories finds all directories containing a composite-resource.yaml file.
func findTestDirectories(filesystem afero.Fs, testDir string) ([]string, error) {
	var testDirs []string

	err := afero.Walk(filesystem, testDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.Name() == CompositeFileName {
			testDirs = append(testDirs, filepath.Dir(path))
		}

		return nil
	})

	return testDirs, err
}

// renderTest renders a single test directory.
func renderTest(ctx context.Context, log logging.Logger, filesystem afero.Fs, dir string) ([]byte, error) {
	log.Debug("Processing test directory", "directory", dir)

	compositeResource, err := loadCompositeResource(filesystem, dir)
	if err != nil {
		return nil, err
	}

	compositionName, err := extractCompositionName(compositeResource, dir)
	if err != nil {
		return nil, err
	}
	log.Debug("Found composition reference", "name", compositionName)

	composition, err := findComposition(filesystem, ".", compositionName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot find composition for %q", compositionName)
	}

	functions, err := render.LoadFunctions(filesystem, FunctionsFileName)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load functions from functions file")
	}

	renderInputs := render.Inputs{
		CompositeResource: compositeResource,
		Composition:       composition,
		Functions:         functions,
		Context:           make(map[string][]byte),
	}

	if err := loadOptionalResources(filesystem, dir, &renderInputs, log); err != nil {
		return nil, err
	}

	outputs, err := render.Render(ctx, log, renderInputs)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot render for %q", dir)
	}

	return marshalOutputs(outputs)
}

// loadCompositeResource loads the composite resource from the test directory.
func loadCompositeResource(filesystem afero.Fs, dir string) (*composite.Unstructured, error) {
	compositeResourceFilePath := filepath.Join(dir, CompositeFileName)
	compositeResource, err := render.LoadCompositeResource(filesystem, compositeResourceFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load CompositeResource from %q", compositeResourceFilePath)
	}
	return compositeResource, nil
}

// extractCompositionName extracts the composition name from the composite resource.
func extractCompositionName(compositeResource *composite.Unstructured, dir string) (string, error) {
	compositionName, found, err := unstructured.NestedString(compositeResource.Object, "spec", "crossplane", "compositionRef", "name")
	if err != nil {
		return "", errors.Wrapf(err, "cannot extract composition name from composite resource in %q", dir)
	}
	if !found {
		return "", errors.Errorf("spec.crossplane.compositionRef.name not found in composite resource in %q", dir)
	}
	return compositionName, nil
}

// findComposition searches for a Composition by name, among the files in the search dir.
func findComposition(filesystem afero.Fs, searchDir, compositionName string) (*v1.Composition, error) {
	var foundComposition *v1.Composition

	err := afero.Walk(filesystem, searchDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Only check .yaml or .yml files
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Try to load as a Composition
		composition, err := render.LoadComposition(filesystem, path)
		if err != nil {
			// Only skip if it's not a composition; other errors should be returned
			if strings.Contains(err.Error(), "not a composition") {
				return nil
			}
			return err
		}

		if composition.Name == compositionName {
			foundComposition = composition
			return filepath.SkipAll // Found it, stop walking
		}

		return nil
	})

	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return nil, err
	}

	if foundComposition == nil {
		return nil, errors.Errorf("composition %q not found", compositionName)
	}

	return foundComposition, nil
}

// loadOptionalResources loads optional extra resources, observed resources, and contexts.
func loadOptionalResources(filesystem afero.Fs, dir string, renderInputs *render.Inputs, log logging.Logger) error {
	if err := loadExtraResources(filesystem, dir, renderInputs, log); err != nil {
		return err
	}

	if err := loadObservedResources(filesystem, dir, renderInputs, log); err != nil {
		return err
	}

	return loadContexts(filesystem, dir, renderInputs, log)
}

// loadExtraResources loads optional extra resources from extra-resources.yaml.
func loadExtraResources(filesystem afero.Fs, dir string, renderInputs *render.Inputs, log logging.Logger) error {
	extraResourcesPath := filepath.Join(dir, ExtraResourcesFileName)
	exists, err := afero.Exists(filesystem, extraResourcesPath)
	if err != nil {
		return errors.Wrapf(err, "cannot check if extra resources file exists at %q", extraResourcesPath)
	}
	if !exists {
		return nil
	}

	extraResources, err := render.LoadRequiredResources(filesystem, extraResourcesPath)
	if err != nil {
		return errors.Wrapf(err, "cannot load extra resources from %q", extraResourcesPath)
	}
	renderInputs.ExtraResources = extraResources
	log.Debug("Loaded extra resources", "path", extraResourcesPath)
	return nil
}

// loadObservedResources loads optional observed resources from observed-resources.yaml.
func loadObservedResources(filesystem afero.Fs, dir string, renderInputs *render.Inputs, log logging.Logger) error {
	observedResourcesPath := filepath.Join(dir, ObservedResourcesFileName)
	exists, err := afero.Exists(filesystem, observedResourcesPath)
	if err != nil {
		return errors.Wrapf(err, "cannot check if observed resources file exists at %q", observedResourcesPath)
	}
	if !exists {
		return nil
	}

	observedResources, err := render.LoadObservedResources(filesystem, observedResourcesPath)
	if err != nil {
		return errors.Wrapf(err, "cannot load observed resources from %q", observedResourcesPath)
	}
	renderInputs.ObservedResources = observedResources
	log.Debug("Loaded observed resources", "path", observedResourcesPath)
	return nil
}

// loadContexts loads optional context files from the contexts directory.
func loadContexts(filesystem afero.Fs, dir string, renderInputs *render.Inputs, log logging.Logger) error {
	contextsDir := filepath.Join(dir, "contexts")
	exists, err := afero.DirExists(filesystem, contextsDir)
	if err != nil {
		return errors.Wrapf(err, "cannot check if contexts directory exists at %q", contextsDir)
	}
	if !exists {
		return nil
	}

	contextFiles, err := afero.ReadDir(filesystem, contextsDir)
	if err != nil {
		return errors.Wrapf(err, "cannot read contexts directory %q", contextsDir)
	}

	for _, fileInfo := range contextFiles {
		if fileInfo.IsDir() || filepath.Ext(fileInfo.Name()) != ".json" {
			continue
		}

		contextFilePath := filepath.Join(contextsDir, fileInfo.Name())
		contextData, err := afero.ReadFile(filesystem, contextFilePath)
		if err != nil {
			return errors.Wrapf(err, "cannot read context file %q", contextFilePath)
		}

		contextName := strings.TrimSuffix(fileInfo.Name(), ".json")
		renderInputs.Context[contextName] = contextData
		log.Debug("Loaded context", "name", contextName, "path", contextFilePath)
	}

	return nil
}

// marshalOutputs converts render outputs to YAML bytes separated by ---.
func marshalOutputs(outputs render.Outputs) ([]byte, error) {
	// Pre-allocate slice with known capacity
	yamlDocs := make([][]byte, 0, len(outputs.ComposedResources)+1)

	xrYAML, err := yaml.Marshal(outputs.CompositeResource.Object)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal composite resource to YAML")
	}
	yamlDocs = append(yamlDocs, xrYAML)

	for _, composed := range outputs.ComposedResources {
		composedYAML, err := yaml.Marshal(composed.Object)
		if err != nil {
			return nil, errors.Wrap(err, "cannot marshal composed resource to YAML")
		}
		yamlDocs = append(yamlDocs, composedYAML)
	}

	// Join with yaml document separator
	return bytes.Join(yamlDocs, []byte("---\n")), nil
}
