package utils

import "testing"

func TestIsOn(t *testing.T) {
	cases := map[string]bool{
		"true":   true,
		"1":      true,
		"TRUE":   true,
		" true ": true,
		"True":   true,
		"yes":    false,
		"false":  false,
		"0":      false,
		"":       false,
		"on":     false,
	}
	for value, want := range cases {
		if got := IsOn(value); got != want {
			t.Errorf("IsOn(%q) = %v, want %v", value, got, want)
		}
	}
}
