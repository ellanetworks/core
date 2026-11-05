// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	goruntime "runtime"
	"syscall"

	"github.com/ellanetworks/core/pkg/runtime"
	"github.com/ellanetworks/core/ui"
)

const (
	mutexProfileFraction = 100
	blockProfileRateNs   = 1_000_000
)

func main() {
	log.SetOutput(os.Stderr)

	goruntime.SetMutexProfileFraction(mutexProfileFraction)
	goruntime.SetBlockProfileRate(blockProfileRateNs)

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
