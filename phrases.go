package main

import (
	"strings"

	"github.com/agnivade/levenshtein"
)

var (
	greetings = []string{"hi", "hello", "how are you", "how are you doing", "alright", "yo", "whats up", "good morning", "morning", "good afternoon", "afternoon", "good evening", "evening", "can you help", "help me", "can you help me", "i need help", "sup"}
)

func isMaybeGreeting(s string) bool {
	s = strings.ToLower(s)
	for _, greeting := range greetings {
		if levenshtein.ComputeDistance(greeting, s) <= 5 {
			return true
		}
	}

	return false
}
