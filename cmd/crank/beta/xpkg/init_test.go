package xpkg

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	_ "embed"
)

//go:embed testdata/NOTES.txt
var notesFile string

func TestHandleNotes(t *testing.T) {
	type args struct {
		file string
	}
	type want struct {
		result string
		err    error
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"PrintsNotes": {
			reason: "Should print the notes file",
			args: args{
				file: notesFile,
			},
			want: want{
				result: fmt.Sprintf("\n%s\n", notesFile),
				err:    nil,
			},
		},
		"NoNotes": {
			reason: "Should not print the notes file when it does not exist",
			args: args{
				file: "",
			},
			want: want{
				result: "",
				err:    nil,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			logger := logging.NewNopLogger()
			dir := t.TempDir()
			if tc.args.file != "" {
				if err := os.WriteFile(filepath.Join(dir, notes), []byte(tc.args.file), 0644); err != nil {
					t.Fatalf("writeFile() error = %v", err)
				}
			}

			c := &initCmd{
				Directory: dir,
			}

			b := &bytes.Buffer{}
			err := c.handleNotes(b, logger)
			if diff := cmp.Diff(tc.want.result, b.String()); diff != "" {
				t.Errorf("\n%s\nInitCmd.handleNotes(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nInitCmd.handleNotes(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
