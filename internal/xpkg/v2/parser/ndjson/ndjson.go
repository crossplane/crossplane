// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ndjson

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// LineReader represents a reader that reads from the underlying reader
// line by line, separated by '\n'.
type LineReader struct {
	reader *bufio.Reader
}

// NewReader returns a new reader, using the underlying io.Reader
// as input.
func NewReader(r *bufio.Reader) *LineReader {
	return &LineReader{reader: r}
}

// Read returns a single line (with '\n' ended) from the underlying reader.
// An error is returned iff there is an error with the underlying reader.
func (r *LineReader) Read() ([]byte, error) {
	for {
		line, err := r.reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}

		// skip blank lines
		if len(line) != 0 && !bytes.Equal(line, []byte{'\n'}) {
			return line, nil
		}

		// EOF seen and there's nothing left in the reader, return EOF.
		if errors.Is(err, io.EOF) {
			return nil, err
		}
	}
}
