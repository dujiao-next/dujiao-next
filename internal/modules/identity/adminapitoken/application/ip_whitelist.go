package adminapitokenapp

import (
	"errors"
	"net/netip"
	"strings"
)

var ErrIPWhitelist = errors.New("invalid admin API token IP whitelist")

const maxIPWhitelistEntries = 100

// Normalize before persistence, including network host bits and mapped IPv4.
// An explicit empty/invalid entry is rejected, never silently dropped.
func normalizeIPWhitelist(entries []string) ([]string, error) {
	if len(entries) > maxIPWhitelistEntries {
		return nil, ErrIPWhitelist
	}
	result := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		prefix, err := parseIPRange(strings.TrimSpace(entry))
		if err != nil {
			return nil, ErrIPWhitelist
		}
		canonical := prefix.String()
		if prefix.IsSingleIP() {
			canonical = prefix.Addr().String()
		}
		if !seen[canonical] {
			result = append(result, canonical)
			seen[canonical] = true
		}
	}
	return result, nil
}

func parseIPRange(entry string) (netip.Prefix, error) {
	if addr, err := netip.ParseAddr(entry); err == nil && addr.Zone() == "" {
		addr = addr.Unmap()
		return netip.PrefixFrom(addr, addr.BitLen()), nil
	}
	prefix, err := netip.ParsePrefix(entry)
	if err != nil {
		return netip.Prefix{}, ErrIPWhitelist
	}
	if prefix.Addr().Is4In6() {
		if prefix.Bits() < 96 {
			return netip.Prefix{}, ErrIPWhitelist
		}
		prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
	}
	return prefix.Masked(), nil
}

// AllowsIP fails closed on invalid persisted rules or an unparseable source.
// A nil/empty list preserves unrestricted access for pre-existing tokens.
func AllowsIP(entries []string, source string) bool {
	if len(entries) == 0 {
		return true
	}
	if len(entries) > maxIPWhitelistEntries {
		return false
	}
	addr, err := netip.ParseAddr(source)
	if err != nil || addr.Zone() != "" {
		return false
	}
	addr = addr.Unmap()
	matched := false
	for _, entry := range entries {
		prefix, err := parseIPRange(entry)
		if err != nil {
			return false
		}
		matched = matched || prefix.Contains(addr)
	}
	return matched
}
