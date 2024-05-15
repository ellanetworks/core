package main

import (
	"flag"
	"fmt"

	"github.com/yeastengine/micro-aether/internal/amf"
)

func main() {

	flag.String("amfcfg", "", "/path/to/amf.yaml")
	flag.Parse()
	amfcfg := flag.Lookup("amfcfg").Value.String()
	if amfcfg == "" {
		panic("amfcfg is required")
	}

	go func() {
		fmt.Printf("Hello")
		err := amf.Start(amfcfg)
		fmt.Printf("bye")
		if err != nil {
			panic(err)
		}
	}()

	select {}
}
