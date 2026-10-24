package application

import (
	"strings"
	"testing"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
)

func TestDisplayNameRules(t *testing.T) {
	xss := `"><img src=x onerror=alert(1)>`
	if userdomain.ValidDisplayName(xss) {
		t.Fatal("xss payload must be rejected")
	}
	if got := userdomain.SanitizeDisplayName(xss); strings.ContainsAny(got, `<>"'`) {
		t.Fatalf("sanitize left html chars: %q", got)
	}
	for _, ok := range []string{"张三", "Alice Bob", "a&b", "🐱 cat"} {
		if !userdomain.ValidDisplayName(ok) {
			t.Fatalf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "a\nb", strings.Repeat("字", userdomain.DisplayNameMaxRunes+1)} {
		if userdomain.ValidDisplayName(bad) {
			t.Fatalf("%q should be invalid", bad)
		}
	}
	if !userdomain.ValidDisplayName(strings.Repeat("字", userdomain.DisplayNameMaxRunes)) {
		t.Fatal("max length should be valid")
	}
}
