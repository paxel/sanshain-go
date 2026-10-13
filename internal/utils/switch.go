package utils

import "strings"

// IsOn reads a SANSHAIN_* boolean switch: a trimmed, case-insensitive "true"
// or "1" means on; anything else, including an empty value, means off.
func IsOn(value string) bool {
	v := strings.TrimSpace(value)
	return v == "1" || strings.EqualFold(v, "true")
}
