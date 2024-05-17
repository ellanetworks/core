package main

import (
	"flag"

	"github.com/yeastengine/micro-aether/internal/amf"
	"github.com/yeastengine/micro-aether/internal/webui"
)

func main() {
	flag.String("amfcfg", "", "/path/to/amf.yaml")
	flag.String("webuicfg", "", "/path/to/webui.yaml")
	flag.Parse()
	amfcfg := flag.Lookup("amfcfg").Value.String()
	if amfcfg == "" {
		panic("amfcfg is required")
	}
	webuicfg := flag.Lookup("webuicfg").Value.String()
	if webuicfg == "" {
		panic("webuicfg is required")
	}

	go func() {
		err := webui.Start(webuicfg)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		err := amf.Start(amfcfg)
		if err != nil {
			panic(err)
		}
	}()

	select {}
}
