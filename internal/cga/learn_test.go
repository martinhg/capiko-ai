package cga

import "testing"

func TestNormalizeDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "mixed case is lowercased",
			input: "Missing Tests For The New Branch",
			want:  "missing tests for the new branch",
		},
		{
			name:  "extra internal whitespace collapses",
			input: "var   named    x   incorrectly",
			want:  "var named x incorrectly",
		},
		{
			name:  "leading and trailing whitespace trimmed",
			input: "   consider a comment   ",
			want:  "consider a comment",
		},
		{
			name:  "trailing punctuation stripped",
			input: "missing test for the new branch.",
			want:  "missing test for the new branch",
		},
		{
			name:  "multiple trailing punctuation stripped",
			input: "is this really ok?!",
			want:  "is this really ok",
		},
		{
			name:  "combined case whitespace and punctuation",
			input: "  Var Named X  Incorrectly.  ",
			want:  "var named x incorrectly",
		},
		{
			name:  "empty string stays empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only becomes empty",
			input: "   \t  ",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeDescription(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeDescription(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
