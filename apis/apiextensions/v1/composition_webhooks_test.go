package v1

import (
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComposition_GetValidationMode(t *testing.T) {
	tests := []struct {
		name    string
		comp    *Composition
		want    CompositionValidationMode
		wantErr bool
	}{
		{
			name: "Default",
			comp: &Composition{
				Spec: CompositionSpec{},
			},
			want: CompositionValidationModeLoose,
		},
		{
			name: "Strict",
			comp: &Composition{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						CompositionValidationModeAnnotation: string(CompositionValidationModeStrict),
					},
				},
			},
			want: CompositionValidationModeStrict,
		},
		{
			name: "Invalid",
			comp: &Composition{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						CompositionValidationModeAnnotation: "invalid",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.comp.GetValidationMode()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValidationMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetValidationMode() got = %v, want %v", got, tt.want)
			}
		})
	}
}
