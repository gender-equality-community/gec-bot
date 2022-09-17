package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dave/jennifer/jen"
)

const output = "config.gen.go"

var config = flag.String("f", "config.toml", "configuration file to load")

// Responses holds our default responses
type Responses struct {
	Greeting   string
	Thankyou   string
	Disclaimer string
}

// Config holds the loaded configuration from our input file
type Config struct {
	Responses Responses
}

func main() {
	flag.Parse()

	c, err := readToml(*config)
	if err != nil {
		panic(err)
	}

	f, err := generate(c)
	if err != nil {
		panic(err)
	}

	fmt.Println(f)
}

func readToml(f string) (c Config, err error) {
	//#nosec
	cf, err := os.ReadFile(f)
	if err != nil {
		return
	}

	_, err = toml.Decode(string(cf), &c)

	return
}

func generate(c Config) (out string, err error) {
	f := jen.NewFile("main")
	f.HeaderComment("Code generated from gen/main.go DO NOT EDIT ")

	f.Comment("Greeting response is sent when a recipient sends a message sends us a greeting")
	f.Const().Id("greetingResponse").Op("=").Lit(c.Responses.Greeting)

	f.Comment("Thank You response is sent when a recipient sends us a message and is capped at a max of 1 per 30 mins")
	f.Const().Id("thankyouResponse").Op("=").Lit(c.Responses.Thankyou)

	f.Comment("Disclaimer response is sent to ensure recipients don't send us stuff we can't deal with.")
	f.Const().Id("disclaimerResponse").Op("=").Lit(c.Responses.Disclaimer)

	buf := strings.Builder{}

	err = f.Render(&buf)
	out = buf.String()

	return
}
