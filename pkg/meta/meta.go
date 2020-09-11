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

package meta

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MetadataOperation func(string, o metav1.Object) error

func AddUISchema(basePath string, o metav1.Object) error {
	filename := path.Join(basePath, fmt.Sprintf("%s.ui-schema.yaml", o.GetName()))
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrapf(err, "cannot read %s", filename)
	}
	meta.AddAnnotations(o, map[string]string{"packages.crossplane.io/ui-schema": string(bytes)})
	return nil
}

func AddIcon(basePath string, o metav1.Object) error {
	filename := path.Join(basePath, fmt.Sprintf("%s.icon.svg", o.GetName()))
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrapf(err, "cannot read %s", filename)
	}
	mediaType := "image/svg+xml"
	b64data := base64.StdEncoding.EncodeToString(bytes)
	meta.AddAnnotations(o, map[string]string{"packages.crossplane.io/icon-data-uri": fmt.Sprintf("data:%s;base64;%s", mediaType, b64data)})
	return nil
}
