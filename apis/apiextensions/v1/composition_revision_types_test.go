package v1_test

import (
	"testing"

	xpv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/resources"
	"github.com/stretchr/testify/require"
	apitest "k8s.io/apiextensions-apiserver/pkg/test"
)

func TestResourceSelectorCELValidation(t *testing.T) {
	testCases := []struct {
		name         string
		current, old map[string]interface{}
		wantErrs     []string
	}{
		{
			name: "none is set",
			current: map[string]interface{}{
				"name":      nil,
				"namespace": nil,
			},
			wantErrs: []string{
				"openAPIV3Schema.properties.spec.properties.permissionClaims.items.properties.resourceSelector.items: Invalid value: \"object\": at least one field must be set",
			},
		},
		{
			name: "namespace is set",
			current: map[string]interface{}{
				"name":      nil,
				"namespace": "foo",
			},
		},
		{
			name: "name is set",
			current: map[string]interface{}{
				"name":      "foo",
				"namespace": nil,
			},
		},
		{
			name: "both name and namespace are set",
			current: map[string]interface{}{
				"name":      "foo",
				"namespace": "bar",
			},
		},
	}

	validators := apitest.FieldValidators(t, resources.Loader.CRDFor(t, &xpv1.FunctionRevision{}))

	for _, tc := range testCases {
		pth := "openAPIV3Schema.properties"
		validator, found := validators["v1alpha1"][pth]
		require.True(t, found, "failed to find validator for %s", pth)

		t.Run(tc.name, func(t *testing.T) {
			errs := validator(tc.current, tc.old)
			t.Log(errs)

			if got := len(errs); got != len(tc.wantErrs) {
				t.Errorf("expected errors %v, got %v", len(tc.wantErrs), len(errs))
				return
			}

			for i := range tc.wantErrs {
				got := errs[i].Error()
				if got != tc.wantErrs[i] {
					t.Errorf("want error %q, got %q", tc.wantErrs[i], got)
				}
			}
		})
	}
}
