package e2engine

import (
	"strings"
	"testing"
)

func TestValidTestExecutionID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{
			name: "valid lowercase hexadecimal",
			id:   strings.Repeat("a1b2c3d4", 8),
			want: true,
		},
		{
			name: "valid lowercase alphanumeric",
			id:   strings.Repeat("abcdefghijklmnopqrstuvwxyz012345", 2),
			want: true,
		},
		{
			name: "valid digits only",
			id:   strings.Repeat("12345678", 8),
			want: true,
		},
		{
			name: "valid letters only",
			id:   strings.Repeat("abcdefgh", 8),
			want: true,
		},
		{
			name: "empty",
			id:   "",
			want: false,
		},
		{
			name: "too short",
			id:   strings.Repeat("a", 63),
			want: false,
		},
		{
			name: "too long",
			id:   strings.Repeat("a", 65),
			want: false,
		},
		{
			name: "uppercase letters",
			id:   strings.Repeat("a", 63) + "A",
			want: false,
		},
		{
			name: "hyphen",
			id:   strings.Repeat("a", 63) + "-",
			want: false,
		},
		{
			name: "underscore",
			id:   strings.Repeat("a", 63) + "_",
			want: false,
		},
		{
			name: "space",
			id:   strings.Repeat("a", 63) + " ",
			want: false,
		},
		{
			name: "leading whitespace",
			id:   " " + strings.Repeat("a", 63),
			want: false,
		},
		{
			name: "newline",
			id:   strings.Repeat("a", 63) + "\n",
			want: false,
		},
		{
			name: "null byte",
			id:   strings.Repeat("a", 63) + "\x00",
			want: false,
		},
		{
			name: "non-ASCII character",
			id:   strings.Repeat("a", 62) + "é",
			want: false,
		},
		{
			name: "Unicode character",
			id:   strings.Repeat("a", 61) + "界",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidTestExecutionID(tt.id)
			if got != tt.want {
				t.Errorf(
					"ValidTestExecutionID(%q) = %t, want %t",
					tt.id,
					got,
					tt.want,
				)
			}
		})
	}
}
