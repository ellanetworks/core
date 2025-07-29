package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ellanetworks/core/pkg/runtime"
	"github.com/ellanetworks/core/ui"
)

func main() {
	log.SetOutput(os.Stderr)

	configFilePtr := flag.String("config", "", "The config file to be provided to the server")
	flag.Parse()

	if *configFilePtr == "" {
		log.Fatal("No config file provided. Use `-config` to provide a config file")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := runtime.Start(ctx, runtime.RuntimeConfig{
		ConfigPath: *configFilePtr,
		EmbedFS:    ui.FrontendFS,
	})
	if err != nil {
		log.Fatal(err)
	}
}
