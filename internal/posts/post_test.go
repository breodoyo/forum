package posts

import (
	"reflect"
	"testing"
)

func TestSanitizeCategories(t *testing.T) {
	cases := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "removes duplicates",
			input: []string{"Tech", "Tech", "News"},
			want:  []string{"Tech", "News"},
		},
		{
			name:  "trims whitespace",
			input: []string{"  Tech  ", "News"},
			want:  []string{"Tech", "News"},
		},
		{
			name:  "drops empty and whitespace-only strings",
			input: []string{"", "   ", "News"},
			want:  []string{"News"},
		},
		{
			name:  "empty input yields nil",
			input: []string{},
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeCategories(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("sanitizeCategories(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}