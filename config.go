package main

import "os"

var (
	// Greeting response is sent when a recipient sends a message sends us a greeting
	greetingResponse = Lookup("GREETING", "Hello, my name is Ada. What's on your mind?")

	// Thank You response is sent when a recipient sends us a message and is capped at a max of 1 per 30 mins
	thankyouResponse = Lookup("THANK_YOU", "Thank you for your message, please provide as much information as you're comfortable sharing and I'll get back to you as soon as I can.")

	// Disclaimer response is sent to ensure recipients don't send us stuff we can't deal with.
	disclaimerResponse = Lookup("DISCLAIMER", "DISCLAIMER: This is not an incident reporting service. If you believe you're being subjected to bullying, harassment, or misconduct then I cannot escalate on your behalf but I can advise you on your next steps.")
)

// Lookup accepts an environment variable name and a default value.
// If the variable exists and is set, then Lookup returns that,
// otherwise Lookup returns the default value
func Lookup(v, d string) string {
	s, ok := os.LookupEnv(v)
	if ok {
		return s
	}
	return d
}
