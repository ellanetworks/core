package main

import (
	"flag"

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
		err := amf.Start(amfcfg)
		if err != nil {
			panic(err)
		}
	}()

	select {}
}
