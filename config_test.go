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

func TestMustLookup(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Error("expected panic, received none")
		}
	}()

	MustLookup("__AN_UNSET_KEY_HOPEFULLY")
}
