package password

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGenerate(t *testing.T) {
	// ¯\_(ツ)_/¯

	want := "aaa"
	got, err := Settings{CharacterSet: "a", Length: 3}.Generate()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Generate(): -want, +got:\n%s", diff)
	}
	if err != nil {
		t.Errorf("Generate: %s\n", err)
	}
}
