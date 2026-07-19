package upstream

import "testing"

func TestMapCallbackStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "delivered keep delivered", input: "delivered", expect: "delivered"},
		{name: "completed keep completed", input: "completed", expect: "completed"},
		{name: "fulfilled keep fulfilled", input: "fulfilled", expect: "fulfilled"},
		{name: "canceled keep canceled", input: "canceled", expect: "canceled"},
		{name: "cancelled map canceled", input: "cancelled", expect: "canceled"},
		{name: "refunded keep refunded", input: "refunded", expect: "refunded"},
		{name: "partially refunded keep value", input: "partially_refunded", expect: "partially_refunded"},
		{name: "trim and lower", input: "  ReFuNdEd  ", expect: "refunded"},
		{name: "fallback normalized raw", input: "PROCESSING", expect: "processing"},
	}

	for _, tc := range tests {
		got := mapCallbackStatus(tc.input)
		if got != tc.expect {
			t.Fatalf("%s: want %q got %q", tc.name, tc.expect, got)
		}
	}
}
