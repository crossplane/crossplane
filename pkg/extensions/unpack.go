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

package extensions

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/afero"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplaneio/crossplane/pkg/apis/extensions/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
)

const (
	registryDirName         = ".registry"
	resourcesDirName        = "resources"
	installFileName         = "install.yaml"
	appFileName             = "app.yaml"
	permissionsFileName     = "rbac.yaml"
	iconFileNamePattern     = "icon.*"
	resourceFileNamePattern = "*crd.yaml"
	yamlSeparator           = "\n---\n"
)

var (
	log = logging.Logger.WithName("extensions")
)

// Unpack unpacks the extension contents from the given directory.
func Unpack(contentDir string) error {
	log.V(logging.Debug).Info("Unpacking extension", "contentDir", contentDir)
	fs := afero.NewOsFs()

	registryRoot := findRegistryRoot(fs, contentDir)

	content, err := doUnpack(fs, registryRoot)
	if err != nil {
		return err
	}

	// write content to stdout
	_, err = os.Stdout.WriteString(content)

	return err
}

func findRegistryRoot(fs afero.Fs, dir string) string {
	if _, err := fs.Stat(filepath.Join(dir, registryDirName)); err == nil {
		// the .registry subdir exists under the given dir, that must be the registry root
		return filepath.Join(dir, registryDirName)
	}

	// didn't find a .registry subdir, the given dir must already be the root
	return dir
}

func doUnpack(fs afero.Fs, root string) (string, error) {
	var output strings.Builder

	// create an Extension record and populate it with the relevant package contents
	extensionRecord := &v1alpha1.Extension{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       strings.Title(v1alpha1.ExtensionKind), // ensure that Kind has an uppercase first letter
		},
	}

	// find all CRDs and add to the extension record and the output builder
	crdList, crdContent, err := readResources(fs, root)
	if err != nil {
		return "", err
	}
	extensionRecord.Spec.CRDs = *crdList

	_, err = output.WriteString(crdContent)
	if err != nil {
		return "", err
	}

	// read the install file information
	installObj := apps.Deployment{}
	if err := readFileIntoObject(fs, root, installFileName, true, &installObj); err != nil {
		return "", err
	}
	extensionRecord.Spec.Controller.Deployment = &v1alpha1.ControllerDeployment{
		Name: installObj.GetName(),
		Spec: installObj.Spec,
	}

	// read the app file information
	appObj := v1alpha1.AppMetadataSpec{}
	if err := readFileIntoObject(fs, root, appFileName, true, &appObj); err != nil {
		return "", err
	}
	extensionRecord.Spec.AppMetadataSpec = appObj

	// read the icon file and encode to base64
	icons, err := readIcons(fs, root)
	if err != nil {
		return "", err
	}
	extensionRecord.Spec.AppMetadataSpec.Icons = icons

	// read the RBAC information
	permissionsObj := v1alpha1.PermissionsSpec{}
	if err := readFileIntoObject(fs, root, permissionsFileName, false /* not required */, &permissionsObj); err != nil {
		return "", err
	}
	extensionRecord.Spec.Permissions = permissionsObj

	// marshal the full extension record to yaml and write it to the output
	extensionRecordRaw, err := yaml.Marshal(extensionRecord)
	if err != nil {
		return "", err
	}
	_, err = output.WriteString(yamlSeparator + string(extensionRecordRaw))
	if err != nil {
		return "", err
	}

	return output.String(), nil
}

func readFileIntoObject(fs afero.Fs, root, fileName string, required bool, obj interface{}) error {
	file, err := fs.Open(filepath.Join(root, fileName))
	if err != nil {
		if os.IsNotExist(err) && !required {
			// the given file doesn't exist, but it's also not required, this is OK
			return nil
		}
		return err
	}
	defer func() { _ = file.Close() }()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(b, obj); err != nil {
		return fmt.Errorf("failed to unmarshal yaml from file %s: %+v", file.Name(), err)
	}

	return nil
}

func readResources(fs afero.Fs, root string) (crdList *v1alpha1.CRDList, crdContent string, err error) {
	resourcesDir := filepath.Join(root, resourcesDirName)

	// check for the existence of the resources directory
	dirExists, err := afero.DirExists(fs, resourcesDir)
	if !dirExists || err != nil {
		// the resources dir doesn't appear to exist, this isn't an error but return nothing
		return v1alpha1.NewCRDList(), "", nil
	}

	// walk the resources dir and find all the CRD files
	resourcesFiles, err := findResourcesFiles(fs, resourcesDir)
	if err != nil {
		return nil, "", err
	}

	// now that we have found all the resource files, process each one in the list
	var sb strings.Builder
	crdList = v1alpha1.NewCRDList()
	for _, rf := range resourcesFiles {
		if err := readResourceFile(fs, rf, crdList, &sb); err != nil {
			return nil, "", err
		}
	}

	return crdList, sb.String(), nil
}

func findResourcesFiles(fs afero.Fs, resourcesDir string) ([]string, error) {
	resourcesFiles := []string{}
	err := afero.Walk(fs, resourcesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// just proceed to next entry on walk
			return nil
		}
		if matched, err := filepath.Match(resourceFileNamePattern, filepath.Base(path)); matched && err == nil {
			// current item on walk matches the resource file name pattern, include it in the list of matches
			resourcesFiles = append(resourcesFiles, path)
		}
		return nil
	})
	if err != nil {
		// walking for resources encountered an error that halted it, surface this error
		return nil, err
	}

	return resourcesFiles, nil
}

func readResourceFile(fs afero.Fs, rf string, crdList *v1alpha1.CRDList, sw io.StringWriter) error {
	b, err := afero.ReadFile(fs, rf)
	if err != nil {
		// we weren't able to read the current resource file, surface this error
		return err
	}

	// unmarshal the raw resource file content into a CRD type
	var crd apiextensions.CustomResourceDefinition
	if err := yaml.Unmarshal(b, &crd); err != nil {
		return err
	}

	// add the CRD type meta to the list
	// TODO(jbw976): handle cases where the CRD has multiple versions associated with it
	crdGVK := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crd.Spec.Version,
		Kind:    crd.Spec.Names.Kind,
	}
	crdTypeMeta := metav1.TypeMeta{
		Kind:       crdGVK.Kind,
		APIVersion: crdGVK.GroupVersion().String(),
	}
	crdList.Owned = append(crdList.Owned, crdTypeMeta)

	// add the raw resource file content to the string builder
	if _, err := sw.WriteString(yamlSeparator + string(b)); err != nil {
		return err
	}

	return nil
}

func readIcons(fs afero.Fs, root string) ([]v1alpha1.IconSpec, error) {
	// look for icon files that start with the standard icon file name pattern
	matches, err := afero.Glob(fs, filepath.Join(root, iconFileNamePattern))
	if err != nil {
		return nil, err
	}

	icons := make([]v1alpha1.IconSpec, len(matches))
	for i, m := range matches {
		// run the loop body in a func so the defer calls happen per loop instead of after the loop
		// https://blog.learngoprogramming.com/gotchas-of-defer-in-go-1-8d070894cb01
		if err := func() error {
			// open the icon file for reading from
			f, err := fs.Open(m)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			// create a base64 encoding byte stream to write to
			b := &bytes.Buffer{}
			w := base64.NewEncoder(base64.StdEncoding, b)
			defer func() { _ = w.Close() }()

			// read from the file stream and write it to the encoding stream
			_, err = io.Copy(w, f)
			if err != nil {
				return err
			}

			// determine the media type of the icon file
			mediaType := mime.TypeByExtension(filepath.Ext(m))

			// save the base64 icon data and the media type to the icons list
			icons[i] = v1alpha1.IconSpec{
				Base64IconData: b.String(),
				MediaType:      mediaType,
			}

			return nil
		}(); err != nil {
			return nil, err
		}
	}

	return icons, nil
}
