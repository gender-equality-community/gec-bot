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

// Config holds the loaded configuration from our input file
type Config struct {
	Responses struct {
		Greeting   string
		Thankyou   string
		Disclaimer string
	}
}

func main() {
	flag.Parse()

	cf, err := os.ReadFile(*config)
	if err != nil {
		panic(err)
	}

	c := new(Config)

	_, err = toml.Decode(string(cf), c)
	if err != nil {
		panic(err)
	}

	f := jen.NewFile("main")
	f.HeaderComment("Code generated from gen/main.go DO NOT EDIT ")

	f.Comment("Greeting response is sent when a recipient sends a message sends us a greeting")
	f.Const().Id("greetingResponse").Op("=").Lit(c.Responses.Greeting)

	f.Comment("Thank You response is sent when a recipient sends us a message and is capped at a max of 1 per 30 mins")
	f.Const().Id("thankyouResponse").Op("=").Lit(c.Responses.Thankyou)

	f.Comment("Disclaimer response is sent to ensure recipients don't send us stuff we can't deal with.")
	f.Const().Id("disclaimerResponse").Op("=").Lit(c.Responses.Disclaimer)

	buf := strings.Builder{}
	f.Render(&buf)

	fmt.Println(buf.String())
}
