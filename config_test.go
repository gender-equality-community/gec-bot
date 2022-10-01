package main

import (
	"os"
	"testing"
)

func TestLookup(t *testing.T) {
	got := Lookup("__AN_UNSET_KEY_HOPEFULLY", "<3")
	if got != "<3" {
		t.Errorf("unexpected value %q", got)
	}
}

func TestLookup_Exists(t *testing.T) {
	os.Setenv("__THIS_KEY_HAS_A_VALUE", "<3")

	got := Lookup("__THIS_KEY_HAS_A_VALUE", "blahblahblah")
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
