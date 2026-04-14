package admin

import "strconv"

func parseIntDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseUintParam(raw string) uint {
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return uint(value)
}
