package main

import (
	"testing"
)

func TestLookup(t *testing.T) {
	got := Lookup("__AN_UNSET_KEY_HOPEFULLY", "<3")
	if got != "<3" {
		t.Errorf("unexpected value %q", got)
	}
}
