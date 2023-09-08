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

// Package fieldpath provides utilities for working with field paths.
//
// Field paths reference a field within a Kubernetes object via a simple string.
// API conventions describe the syntax as "standard JavaScript syntax for
// accessing that field, assuming the JSON object was transformed into a
// JavaScript object, without the leading dot, such as metadata.name".
//
// Valid examples:
//
// * metadata.name
// * spec.containers[0].name
// * data[.config.yml]
// * metadata.annotations['crossplane.io/external-name']
// * spec.items[0][8]
// * apiVersion
// * [42]
//
// Invalid examples:
//
// * .metadata.name - Leading period.
// * metadata..name - Double period.
// * metadata.name. - Trailing period.
// * spec.containers[] - Empty brackets.
// * spec.containers.[0].name - Period before open bracket.
//
// https://github.com/kubernetes/community/blob/61f3d0/contributors/devel/sig-architecture/api-conventions.md#selecting-fields
package fieldpath

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// A SegmentType within a field path; either a field within an object, or an
// index within an array.
type SegmentType int

// Segment types.
const (
	_ SegmentType = iota
	SegmentField
	SegmentIndex
)

// A Segment of a field path.
type Segment struct {
	Type  SegmentType
	Field string
	Index uint
}

// Segments of a field path.
type Segments []Segment

func (sg Segments) String() string {
	var b strings.Builder

	for _, s := range sg {
		switch s.Type {
		case SegmentField:
			if s.Field == wildcard || strings.ContainsRune(s.Field, period) {
				b.WriteString(fmt.Sprintf("[%s]", s.Field))
				continue
			}
			b.WriteString(fmt.Sprintf(".%s", s.Field))
		case SegmentIndex:
			b.WriteString(fmt.Sprintf("[%d]", s.Index))
		}
	}

	return strings.TrimPrefix(b.String(), ".")
}

// FieldOrIndex produces a new segment from the supplied string. The segment is
// considered to be an array index if the string can be interpreted as an
// unsigned 32 bit integer. Anything else is interpreted as an object field
// name.
func FieldOrIndex(s string) Segment {
	// Attempt to parse the segment as an unsigned integer. If the integer is
	// larger than 2^32 (the limit for most JSON arrays) we presume it's too big
	// to be an array index, and is thus a field name.
	if i, err := strconv.ParseUint(s, 10, 32); err == nil {
		return Segment{Type: SegmentIndex, Index: uint(i)}
	}

	// If the segment is not a valid unsigned integer we presume it's
	// a string field name.
	return Field(s)
}

// Field produces a new segment from the supplied string. The segment is always
// considered to be an object field name.
func Field(s string) Segment {
	return Segment{Type: SegmentField, Field: strings.Trim(s, "'\"")}
}

// Parse the supplied path into a slice of Segments.
func Parse(path string) (Segments, error) {
	l := &lexer{input: path, items: make(chan item)}
	go l.run()

	segments := make(Segments, 0, 1)
	for i := range l.items {
		switch i.typ { //nolint:exhaustive // We're only worried about names, not separators.
		case itemField:
			segments = append(segments, Field(i.val))
		case itemFieldOrIndex:
			segments = append(segments, FieldOrIndex(i.val))
		case itemError:
			return nil, errors.Errorf("%s at position %d", i.val, i.pos)
		}
	}
	return segments, nil
}

const (
	period       = '.'
	leftBracket  = '['
	rightBracket = ']'

	wildcard = "*"
)

type itemType int

const (
	itemError itemType = iota
	itemPeriod
	itemLeftBracket
	itemRightBracket
	itemField
	itemFieldOrIndex
	itemEOL
)

type item struct {
	typ itemType
	pos int
	val string
}

type stateFn func(*lexer) stateFn

// A simplified version of the text/template lexer.
// https://github.com/golang/go/blob/6396bc9d/src/text/template/parse/lex.go#L108
type lexer struct {
	input string
	pos   int
	start int
	items chan item
}

func (l *lexer) run() {
	for state := lexField; state != nil; {
		state = state(l)
	}
	close(l.items)
}

func (l *lexer) emit(t itemType) {
	// Don't emit empty values.
	if l.pos <= l.start {
		return
	}
	l.items <- item{typ: t, pos: l.start, val: l.input[l.start:l.pos]}
	l.start = l.pos
}

func (l *lexer) errorf(pos int, format string, args ...any) stateFn {
	l.items <- item{typ: itemError, pos: pos, val: fmt.Sprintf(format, args...)}
	return nil
}

func lexField(l *lexer) stateFn {
	for i, r := range l.input[l.pos:] {
		switch r {
		// A right bracket may not appear in an object field name.
		case rightBracket:
			return l.errorf(l.pos+i, "unexpected %q", rightBracket)

		// A left bracket indicates the end of the field name.
		case leftBracket:
			l.pos += i
			l.emit(itemField)
			return lexLeftBracket

		// A period indicates the end of the field name.
		case period:
			l.pos += i
			l.emit(itemField)
			return lexPeriod
		}
	}

	// The end of the input indicates the end of the field name.
	l.pos = len(l.input)
	l.emit(itemField)
	l.emit(itemEOL)
	return nil
}

func lexPeriod(l *lexer) stateFn {
	// A period may not appear at the beginning or the end of the input.
	if l.pos == 0 || l.pos == len(l.input)-1 {
		return l.errorf(l.pos, "unexpected %q", period)
	}

	l.pos += utf8.RuneLen(period)
	l.emit(itemPeriod)

	// A period may only be followed by a field name. We defer checking for
	// right brackets to lexField, where they are invalid.
	r, _ := utf8.DecodeRuneInString(l.input[l.pos:])
	if r == period {
		return l.errorf(l.pos, "unexpected %q", period)
	}
	if r == leftBracket {
		return l.errorf(l.pos, "unexpected %q", leftBracket)
	}

	return lexField
}

func lexLeftBracket(l *lexer) stateFn {
	// A right bracket must appear before the input ends.
	if !strings.ContainsRune(l.input[l.pos:], rightBracket) {
		return l.errorf(l.pos, "unterminated %q", leftBracket)
	}

	l.pos += utf8.RuneLen(leftBracket)
	l.emit(itemLeftBracket)
	return lexFieldOrIndex
}

// Strings between brackets may be either a field name or an array index.
// Periods have no special meaning in this context.
func lexFieldOrIndex(l *lexer) stateFn {
	// We know a right bracket exists before EOL thanks to the preceding
	// lexLeftBracket.
	rbi := strings.IndexRune(l.input[l.pos:], rightBracket)

	// A right bracket may not immediately follow a left bracket.
	if rbi == 0 {
		return l.errorf(l.pos, "unexpected %q", rightBracket)
	}

	// A left bracket may not appear before the next right bracket.
	if lbi := strings.IndexRune(l.input[l.pos:l.pos+rbi], leftBracket); lbi > -1 {
		return l.errorf(l.pos+lbi, "unexpected %q", leftBracket)
	}

	// Periods are not considered field separators when we're inside brackets.
	l.pos += rbi
	l.emit(itemFieldOrIndex)
	return lexRightBracket
}

func lexRightBracket(l *lexer) stateFn {
	l.pos += utf8.RuneLen(rightBracket)
	l.emit(itemRightBracket)
	return lexField
}
