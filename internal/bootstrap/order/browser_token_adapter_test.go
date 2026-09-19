package orderwiring

import (
	"os"
	"strings"
	"testing"
)

func TestOrderCreateAdapterForwardsBrowserTokenHash(t *testing.T) {
	source, err := os.ReadFile("adapters.go")
	if err != nil {
		t.Fatalf("read adapters.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func (a orderCreateAdapter) CreateGuestOrder")
	if start < 0 {
		t.Fatal("CreateGuestOrder adapter not found")
	}
	end := strings.Index(text[start:], "\n}\n")
	if end < 0 {
		t.Fatal("CreateGuestOrder adapter end not found")
	}
	body := text[start : start+end]
	if !strings.Contains(body, "BrowserTokenHash:    input.BrowserTokenHash") {
		t.Fatal("guest order adapter drops BrowserTokenHash before the application service")
	}
}
