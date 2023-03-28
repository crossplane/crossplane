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

package composite

import (
	"crypto/sha1" //nolint:gosec // Not used for secure hashing
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

const (
	errMathNoMultiplier   = "no input is given"
	errMathInputNonNumber = "input is required to be a number for math transformer"

	errFmtRequiredField          = "%s is required by type %s"
	errFmtTransformAtIndex       = "transform at index %d returned error"
	errFmtTypeNotSupported       = "transform type %s is not supported"
	errFmtTransformConfigMissing = "given transform type %s requires configuration"
	errFmtTransformTypeFailed    = "%s transform could not resolve"
	errFmtMapTypeNotSupported    = "type %s is not supported for map transform"
	errFmtMapNotFound            = "key %s is not found in map"
	errFmtMapInvalidJSON         = "value for key %s is not valid JSON"

	errFmtMatchPattern            = "cannot match pattern at index %d"
	errFmtMatchParseResult        = "cannot parse result of pattern at index %d"
	errMatchParseFallbackValue    = "cannot parse fallback value"
	errFmtMatchPatternTypeInvalid = "unsupported pattern type '%s'"
	errFmtMatchInputTypeInvalid   = "unsupported input type '%s'"
	errMatchRegexpCompile         = "cannot compile regexp"

	errStringTransformTypeFailed        = "type %s is not supported for string transform type"
	errStringTransformTypeFormat        = "string transform of type %s fmt is not set"
	errStringTransformTypeConvert       = "string transform of type %s convert is not set"
	errStringTransformTypeTrim          = "string transform of type %s trim is not set"
	errStringTransformTypeRegexp        = "string transform of type %s regexp is not set"
	errStringTransformTypeRegexpFailed  = "could not compile regexp"
	errStringTransformTypeRegexpNoMatch = "regexp %q had no matches for group %d"
	errStringConvertTypeFailed          = "type %s is not supported for string convert"
	errFmtConvertInputTypeNotSupported  = "invalid input type %T"

	errConvertFormatPairNotSupported = "conversion from %s to %s is not supported with format %s"

	errDecodeString = "string is not valid base64"
	errMarshalJSON  = "cannot marshal to JSON"
	errHash         = "cannot generate hash"
)

// Resolve the supplied Transform.
func Resolve(t v1.Transform, input any) (any, error) { //nolint:gocyclo // This is a long but simple/same-y switch.
	var out any
	var err error

	switch t.Type {
	case v1.TransformTypeMath:
		if t.Math == nil {
			return nil, errors.Errorf(errFmtTransformConfigMissing, t.Type)
		}
		out, err = ResolveMath(*t.Math, input)
	case v1.TransformTypeMap:
		if t.Map == nil {
			return nil, errors.Errorf(errFmtTransformConfigMissing, t.Type)
		}
		out, err = ResolveMap(*t.Map, input)
	case v1.TransformTypeMatch:
		if t.Match == nil {
			return nil, errors.Errorf(errFmtTransformConfigMissing, t.Type)
		}
		out, err = ResolveMatch(*t.Match, input)
	case v1.TransformTypeString:
		if t.String == nil {
			return nil, errors.Errorf(errFmtTransformConfigMissing, t.Type)
		}
		out, err = ResolveString(*t.String, input)
	case v1.TransformTypeConvert:
		if t.Convert == nil {
			return nil, errors.Errorf(errFmtTransformConfigMissing, t.Type)
		}
		out, err = ResolveConvert(*t.Convert, input)
	default:
		return nil, errors.Errorf(errFmtTypeNotSupported, string(t.Type))
	}

	return out, errors.Wrapf(err, errFmtTransformTypeFailed, string(t.Type))
}

// ResolveMath resolves a Math transform.
func ResolveMath(t v1.MathTransform, input any) (any, error) {
	// Currently we support only multiply.
	if t.Multiply == nil {
		return nil, errors.New(errMathNoMultiplier)
	}
	switch i := input.(type) {
	case int64:
		return *t.Multiply * i, nil
	case int:
		return *t.Multiply * int64(i), nil
	default:
		return nil, errors.New(errMathInputNonNumber)
	}
}

// ResolveMap resolves a Map transform.
func ResolveMap(t v1.MapTransform, input any) (any, error) {
	switch i := input.(type) {
	case string:
		p, ok := t.Pairs[i]
		if !ok {
			return nil, errors.Errorf(errFmtMapNotFound, i)
		}
		var val interface{}
		if err := json.Unmarshal(p.Raw, &val); err != nil {
			return nil, errors.Wrapf(err, errFmtMapInvalidJSON, i)
		}
		return val, nil
	default:
		return nil, errors.Errorf(errFmtMapTypeNotSupported, reflect.TypeOf(input).String())
	}
}

// ResolveMatch resolves a Match transform.
func ResolveMatch(t v1.MatchTransform, input any) (any, error) {
	var output any
	for i, p := range t.Patterns {
		matches, err := Matches(p, input)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtMatchPattern, i)
		}
		if matches {
			if err := unmarshalJSON(p.Result, &output); err != nil {
				return nil, errors.Wrapf(err, errFmtMatchParseResult, i)
			}
			return output, nil
		}
	}
	// Use fallback value if no pattern matches (or if there are no patterns)
	if err := unmarshalJSON(t.FallbackValue, &output); err != nil {
		return nil, errors.Wrap(err, errMatchParseFallbackValue)
	}
	return output, nil
}

// Matches returns true if the pattern matches the supplied input.
func Matches(p v1.MatchTransformPattern, input any) (bool, error) {
	switch p.Type {
	case v1.MatchTransformPatternTypeLiteral:
		return matchesLiteral(p, input)
	case v1.MatchTransformPatternTypeRegexp:
		return matchesRegexp(p, input)
	}
	return false, errors.Errorf(errFmtMatchPatternTypeInvalid, string(p.Type))
}

func matchesLiteral(p v1.MatchTransformPattern, input any) (bool, error) {
	if p.Literal == nil {
		return false, errors.Errorf(errFmtRequiredField, "literal", v1.MatchTransformPatternTypeLiteral)
	}
	inputStr, ok := input.(string)
	if !ok {
		return false, errors.Errorf(errFmtMatchInputTypeInvalid, reflect.TypeOf(input).String())
	}
	return inputStr == *p.Literal, nil
}

func matchesRegexp(p v1.MatchTransformPattern, input any) (bool, error) {
	if p.Regexp == nil {
		return false, errors.Errorf(errFmtRequiredField, "regexp", v1.MatchTransformPatternTypeRegexp)
	}
	re, err := regexp.Compile(*p.Regexp)
	if err != nil {
		return false, errors.Wrap(err, errMatchRegexpCompile)
	}
	if input == nil {
		return false, errors.Errorf(errFmtMatchInputTypeInvalid, "null")
	}
	inputStr, ok := input.(string)
	if !ok {
		return false, errors.Errorf(errFmtMatchInputTypeInvalid, reflect.TypeOf(input).String())
	}
	return re.MatchString(inputStr), nil
}

// unmarshalJSON is a small utility function that returns nil if j contains no
// data. json.Unmarshal seems to not be able to handle this.
func unmarshalJSON(j extv1.JSON, output *any) error {
	if len(j.Raw) == 0 {
		return nil
	}
	return json.Unmarshal(j.Raw, output)
}

// ResolveString resolves a String transform.
func ResolveString(t v1.StringTransform, input any) (string, error) {
	switch t.Type {
	case v1.StringTransformTypeFormat:
		if t.Format == nil {
			return "", errors.Errorf(errStringTransformTypeFormat, string(t.Type))
		}
		return fmt.Sprintf(*t.Format, input), nil
	case v1.StringTransformTypeConvert:
		if t.Convert == nil {
			return "", errors.Errorf(errStringTransformTypeConvert, string(t.Type))
		}
		return stringConvertTransform(t.Convert, input)
	case v1.StringTransformTypeTrimPrefix, v1.StringTransformTypeTrimSuffix:
		if t.Trim == nil {
			return "", errors.Errorf(errStringTransformTypeTrim, string(t.Type))
		}
		return stringTrimTransform(input, t.Type, *t.Trim), nil
	case v1.StringTransformTypeRegexp:
		if t.Regexp == nil {
			return "", errors.Errorf(errStringTransformTypeRegexp, string(t.Type))
		}
		return stringRegexpTransform(input, *t.Regexp)
	default:
		return "", errors.Errorf(errStringTransformTypeFailed, string(t.Type))
	}
}

func stringConvertTransform(t *v1.StringConversionType, input any) (string, error) {
	str := fmt.Sprintf("%v", input)
	switch *t {
	case v1.StringConversionTypeToUpper:
		return strings.ToUpper(str), nil
	case v1.StringConversionTypeToLower:
		return strings.ToLower(str), nil
	case v1.StringConversionTypeToJSON:
		raw, err := json.Marshal(input)
		return string(raw), errors.Wrap(err, errMarshalJSON)
	case v1.StringConversionTypeToBase64:
		return base64.StdEncoding.EncodeToString([]byte(str)), nil
	case v1.StringConversionTypeFromBase64:
		s, err := base64.StdEncoding.DecodeString(str)
		return string(s), errors.Wrap(err, errDecodeString)
	case v1.StringConversionTypeToSHA1:
		hash, err := stringGenerateHash(input, sha1.Sum)
		return hex.EncodeToString(hash[:]), errors.Wrap(err, errHash)
	case v1.StringConversionTypeToSHA256:
		hash, err := stringGenerateHash(input, sha256.Sum256)
		return hex.EncodeToString(hash[:]), errors.Wrap(err, errHash)
	case v1.StringConversionTypeToSHA512:
		hash, err := stringGenerateHash(input, sha512.Sum512)
		return hex.EncodeToString(hash[:]), errors.Wrap(err, errHash)
	default:
		return "", errors.Errorf(errStringConvertTypeFailed, *t)
	}
}

func stringGenerateHash[THash any](input any, hashFunc func([]byte) THash) (THash, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		var ret THash
		return ret, errors.Wrap(err, errMarshalJSON)
	}
	return hashFunc(inputJSON), nil
}

func stringTrimTransform(input any, t v1.StringTransformType, trim string) string {
	str := fmt.Sprintf("%v", input)
	if t == v1.StringTransformTypeTrimPrefix {
		return strings.TrimPrefix(str, trim)
	}
	if t == v1.StringTransformTypeTrimSuffix {
		return strings.TrimSuffix(str, trim)
	}
	return str
}

func stringRegexpTransform(input any, r v1.StringTransformRegexp) (string, error) {
	re, err := regexp.Compile(r.Match)
	if err != nil {
		return "", errors.Wrap(err, errStringTransformTypeRegexpFailed)
	}

	groups := re.FindStringSubmatch(fmt.Sprintf("%v", input))

	// Return the entire match (group zero) by default.
	g := pointer.IntDeref(r.Group, 0)
	if len(groups) == 0 || g >= len(groups) {
		return "", errors.Errorf(errStringTransformTypeRegexpNoMatch, r.Match, g)
	}

	return groups[g], nil
}

// ResolveConvert resolves a Convert transform by looking up the appropriate
// conversion function for the given input type and invoking it.
func ResolveConvert(t v1.ConvertTransform, input any) (any, error) {
	from := v1.TransformIOType(reflect.TypeOf(input).Kind().String())
	if !from.IsValid() {
		return nil, errors.Errorf(errFmtConvertInputTypeNotSupported, input)
	}
	f, err := t.GetConversionFunc(from)
	if err != nil {
		return nil, err
	}
	return f(input)
}
