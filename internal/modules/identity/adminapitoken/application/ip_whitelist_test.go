package adminapitokenapp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeIPWhitelist(t *testing.T) {
	got, err := normalizeIPWhitelist([]string{
		" 192.0.2.1 ", "192.0.2.1/32", "::ffff:192.0.2.1",
		"198.51.100.42/24", "::ffff:198.51.100.1/120",
		"2001:DB8::123/32", "2001:db8::1/128",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"192.0.2.1", "198.51.100.0/24", "2001:db8::/32", "2001:db8::1"}, got)
	empty, err := normalizeIPWhitelist(nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Empty(t, empty)
}

func TestIPWhitelistRejectsInvalidRules(t *testing.T) {
	for _, entry := range []string{
		"", " ", "example.com", "*", "192.0.2.*", "192.0.2.1:443",
		"192.0.2.1/33", "2001:db8::/129", "192.0.2.1/-1",
		"fe80::1%eth0", "fe80::1%eth0/64", "::ffff:192.0.2.1/95",
	} {
		t.Run(entry, func(t *testing.T) {
			_, err := normalizeIPWhitelist([]string{entry})
			require.ErrorIs(t, err, ErrIPWhitelist)
		})
	}
	_, err := normalizeIPWhitelist(strings.Fields(strings.Repeat("192.0.2.1 ", 101)))
	require.ErrorIs(t, err, ErrIPWhitelist)
}

func TestIPWhitelistAddressFamiliesAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		rules   []string
		source  string
		allowed bool
	}{
		{nil, "203.0.113.1", true},
		{[]string{"192.0.2.1"}, "192.0.2.1", true},
		{[]string{"192.0.2.1"}, "192.0.2.2", false},
		{[]string{"192.0.2.0/24"}, "192.0.2.0", true},
		{[]string{"192.0.2.0/24"}, "192.0.2.255", true},
		{[]string{"192.0.2.0/24"}, "192.0.3.0", false},
		{[]string{"192.0.2.0/24"}, "::ffff:192.0.2.42", true},
		{[]string{"2001:db8::/32"}, "2001:db8:ffff::1", true},
		{[]string{"2001:db8::/32"}, "2001:db9::1", false},
		{[]string{"2001:db8::1"}, "2001:db8::2", false},
		{[]string{"::/0"}, "192.0.2.1", false},
		{[]string{"0.0.0.0/0"}, "2001:db8::1", false},
		{[]string{"192.0.2.1"}, "", false},
		{[]string{"192.0.2.1"}, "192.0.2.1:1234", false},
		{[]string{"fe80::/10"}, "fe80::1%eth0", false},
		{[]string{"192.0.2.1", "bad"}, "192.0.2.1", false},
		{[]string{""}, "192.0.2.1", false},
	} {
		require.Equal(t, tc.allowed, AllowsIP(tc.rules, tc.source), "rules=%v source=%s", tc.rules, tc.source)
	}
}
