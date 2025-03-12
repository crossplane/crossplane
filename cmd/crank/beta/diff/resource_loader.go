package diff

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"io"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
)

// TODO:  we should reuse Loader from beta/validate/loader.go instead of rolling our own
func LoadResources(files []string) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured

	// If no files are specified, or if "--" is the last argument, read from stdin
	if len(files) == 0 || (len(files) > 0 && files[len(files)-1] == "--") {
		// Remove the "--" argument if present
		if len(files) > 0 && files[len(files)-1] == "--" {
			files = files[:len(files)-1]
		}
		res, err := loadFromReader(os.Stdin)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read resources from stdin")
		}
		resources = append(resources, res...)
		if len(files) == 0 {
			return resources, nil
		}
	}

	// Read from each specified file
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot open file %q", file)
		}

		res, err := loadFromReader(f)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read resources from file %q", file)
		}

		err = f.Close()
		if err != nil {
			return nil, errors.Wrapf(err, "cannot close file %q", file)
		}

		resources = append(resources, res...)
	}

	return resources, nil
}

func loadFromReader(r io.Reader) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(r, 4096)

	for {
		// Create a new map for each document
		obj := make(map[string]interface{})
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrap(err, "cannot decode YAML")
		}

		// Skip empty documents
		if len(obj) == 0 {
			continue
		}

		u := &unstructured.Unstructured{Object: obj}
		if u.GetAPIVersion() == "" || u.GetKind() == "" {
			return nil, errors.New("resource is missing apiVersion and/or kind")
		}

		resources = append(resources, u)
	}

	if len(resources) == 0 {
		return nil, errors.New("no resources found")
	}

	return resources, nil
}
