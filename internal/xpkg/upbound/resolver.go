// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package upbound

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/alecthomas/kong"
)

// JSON returns a Resolver that retrieves values from a JSON source.
// Based slightly off of https://github.com/alecthomas/kong/blob/f48da244f54370c0cb63e22b0e500e5459a491bf/resolver.go#L33-L60
// Hyphens in flag names are replaced with underscores.
func JSON(base, overlay io.Reader) (kong.Resolver, error) {
	baseValues := map[string]interface{}{}
	overlayValues := map[string]interface{}{}
	err := json.NewDecoder(base).Decode(&baseValues)
	if err != nil {
		return nil, err
	}
	err = json.NewDecoder(overlay).Decode(&overlayValues)
	if err != nil {
		return nil, err
	}

	var f kong.ResolverFunc = func(context *kong.Context, parent *kong.Path, flag *kong.Flag) (interface{}, error) {
		name := strings.ReplaceAll(flag.Name, "-", "_")
		bRaw, bOk := resolveValue(name, flag.Envs, baseValues)
		oRaw, oOk := resolveValue(name, flag.Envs, overlayValues)

		// if found in base and in overlay AND is not the defaultValue for overlay
		if bOk && oOk && flag.Default != oRaw {
			return oRaw, nil
		}

		if bOk {
			return bRaw, nil
		}

		if oOk {
			return oRaw, nil
		}

		return nil, nil
	}

	return f, nil
}

func resolveValue(fieldName string, envVarsName []string, vals map[string]interface{}) (interface{}, bool) {
	// attempt to lookup by field name first
	raw, ok := vals[fieldName]
	if !ok {
		// fall back to env var name
		for _, envVarName := range envVarsName {
			raw, ok = vals[envVarName]
			if ok {
				break
			}
		}
		if !ok {
			return nil, false
		}
	}
	return raw, true
}
