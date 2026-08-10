package application

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeGuestPassword(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "empty", input: "   ", wantErr: ErrGuestPasswordRequired},
		{name: "short", input: "short-pass", wantErr: ErrGuestPasswordTooWeak},
		{name: "minimum ASCII", input: "  123456789012  ", want: "123456789012"},
		{name: "minimum Unicode", input: strings.Repeat("密", MinGuestPasswordLength), want: strings.Repeat("密", MinGuestPasswordLength)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeGuestPassword(test.input)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("password = %q, want %q", got, test.want)
			}
		})
	}
}
