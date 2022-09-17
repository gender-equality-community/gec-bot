package main

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	happyConfig = Config{
		Responses: Responses{
			Greeting:   "Hi there!",
			Thankyou:   "Thanks!",
			Disclaimer: "We can't do that!",
		},
	}
)

func TestReadToml(t *testing.T) {
	for _, test := range []struct {
		name        string
		fn          string
		expect      Config
		expectError bool
	}{
		{"Config file does not exist", "testdata/nonsuch.toml", Config{}, true},
		{"Config file exists but is empty", "testdata/empty.toml", Config{}, false},
		{"Config file exists and has content", "testdata/config.toml", happyConfig, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := readToml(test.fn)
			if err != nil && !test.expectError {
				t.Errorf("unexpected error: %v", err)
			} else if err == nil && test.expectError {
				t.Error("expected error")
			}

			if !reflect.DeepEqual(test.expect, got) {
				t.Errorf("expected %#v, received %#v", test.expect, got)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	expect := `// Code generated from gen/main.go DO NOT EDIT

package main

// Greeting response is sent when a recipient sends a message sends us a greeting
const greetingResponse = "Hi there!"

// Thank You response is sent when a recipient sends us a message and is capped at a max of 1 per 30 mins
const thankyouResponse = "Thanks!"

// Disclaimer response is sent to ensure recipients don't send us stuff we can't deal with.
const disclaimerResponse = "We can't do that!"
`
	got, err := generate(happyConfig)
	if err != nil {
		t.Errorf("expected error")
	}

	if expect != got {
		t.Errorf(cmp.Diff(expect, got))
	}
}

func TestMain(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Error("expected error")
		}
	}()

	main()
}
