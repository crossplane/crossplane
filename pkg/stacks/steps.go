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

package stacks

import (
	"encoding/base64"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/stacks/walker"
)

// installStep unmarshals install.yaml bytes to API machinery Unstructured which is set on the StackPackager
func installStep(sp StackPackager) walker.Step {
	return func(path string, b []byte) error {
		install := unstructured.Unstructured{}
		if err := yaml.Unmarshal(b, &install); err != nil {
			return errors.Wrap(err, fmt.Sprintf("invalid install %q", path))
		}

		err := sp.SetInstall(install)
		return err
	}
}

// iconStep unmarshals icon.* bytes to IconSpec which is added to the StackPackager
func iconStep(sp StackPackager) walker.Step {
	return func(path string, b []byte) error {
		mediaType := mime.TypeByExtension(filepath.Ext(path))
		b64data := base64.StdEncoding.EncodeToString(b)
		icon := v1alpha1.IconSpec{
			Base64IconData: b64data,
			MediaType:      mediaType,
		}

		sp.AddIcon(path, icon)
		return nil
	}
}

// appStep unmarshals app.yaml bytes to AppMetadataSpec which is set on the StackPackager
func appStep(sp StackPackager) walker.Step {
	return func(path string, b []byte) error {
		app := v1alpha1.AppMetadataSpec{}
		if err := yaml.Unmarshal(b, &app); err != nil {
			return errors.Wrap(err, fmt.Sprintf("invalid app %q", path))
		}

		sp.SetApp(app)
		return nil
	}
}

// crdStep unmarshals crd.yaml bytes to a Kubernetes CRD which is added to the StackPackager
func crdStep(sp StackPackager) walker.Step {
	return func(path string, b []byte) error {
		crd := &apiextensions.CustomResourceDefinition{}

		if err := yaml.Unmarshal(b, crd); err != nil {
			return errors.Wrap(err, fmt.Sprintf("invalid crd %q", path))
		}

		if (crd.Spec.Scope != apiextensions.NamespaceScoped) && (crd.Spec.Scope != "") {
			return errors.New(fmt.Sprintf("Stack CRD %q must be namespaced scope", path))
		}

		sp.AddCRD(filepath.Dir(path), crd)
		return nil
	}
}

// groupStep unmarshals group.yaml bytes to a StackGroup which is added to the StackPackager
func groupStep(sp StackPackager) walker.Step {
	return func(path string, b []byte) error {
		sg := &StackGroup{}
		if err := yaml.Unmarshal(b, sg); err != nil {
			return errors.Wrap(err, fmt.Sprintf("invalid group %q", path))
		}

		sp.AddGroup(filepath.Dir(path), *sg)
		return nil
	}
	// TODO(displague) early thinking was that groupStep would return filepath.SkipDir
	// and trigger an inner walker.  Is that approach worth more investigation, or is the
	// current implementation fine (walking maps afterward via applyAnnotations).
}

// resourceStep unmarshals resource.yaml bytes to a StackResource which is added to the StackPackager
func resourceStep(sp StackPackager) walker.Step {
	return func(path string, b []byte) error {
		sr := &StackResource{}
		if err := yaml.Unmarshal(b, sr); err != nil {
			return errors.Wrap(err, fmt.Sprintf("invalid resource %q", path))
		}

		sp.AddResource(path, *sr)
		return nil
	}
}

// uiStep adds ui-schema.yaml bytes to the StackPackager
func uiStep(sp StackPackager) walker.Step {
	return func(path string, b []byte) error {
		sp.AddUI(path, string(b))
		return nil
	}
}
