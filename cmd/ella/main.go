package main

import (
	"flag"
	"log"
	"os"

	"github.com/yeastengine/ella/internal/amf"
	"github.com/yeastengine/ella/internal/ausf"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/db/mongodb"
	"github.com/yeastengine/ella/internal/db/sql"
	"github.com/yeastengine/ella/internal/nssf"
	"github.com/yeastengine/ella/internal/pcf"
	"github.com/yeastengine/ella/internal/server"
	"github.com/yeastengine/ella/internal/smf"
	"github.com/yeastengine/ella/internal/udm"
	"github.com/yeastengine/ella/internal/udr"
	"github.com/yeastengine/ella/internal/upf"
)

func setEnvironmentVariables() error {
	err := os.Setenv("POD_IP", "0.0.0.0")
	if err != nil {
		return err
	}
	err = os.Setenv("PFCP_PORT_UPF", "8806")
	if err != nil {
		return err
	}
	return nil
}

func startNetwork(cfg config.Config, dbQueries *sql.Queries) error {
	ausfUrl := "http://127.0.0.1:29509"
	amfUrl := "http://127.0.0.1:29518"
	nssfUrl := "http://127.0.0.1:29531"
	pcfUrl := "http://127.0.0.1:29507"
	smfUrl := "http://127.0.0.1:29502"
	udmUrl := "http://127.0.0.1:29503"
	udrUrl := "http://127.0.0.1:29504"
	err := server.Start(cfg.Port, cfg.TLS.Cert, cfg.TLS.Key, dbQueries)
	if err != nil {
		return err
	}
	err = amf.Start(ausfUrl, nssfUrl, pcfUrl, smfUrl, udmUrl, udmUrl, dbQueries)
	if err != nil {
		return err
	}
	err = ausf.Start(udmUrl)
	if err != nil {
		return err
	}
	err = pcf.Start(amfUrl)
	if err != nil {
		return err
	}
	err = udr.Start(cfg.DB.Mongo.Url, cfg.DB.Mongo.Name)
	if err != nil {
		return err
	}
	err = udm.Start(udrUrl)
	if err != nil {
		return err
	}
	err = nssf.Start(dbQueries)
	if err != nil {
		return err
	}
	err = smf.Start(amfUrl, pcfUrl, udmUrl, dbQueries)
	if err != nil {
		return err
	}
	err = upf.Start(cfg.UPF.Interfaces, cfg.UPF.N3Address)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	log.SetOutput(os.Stderr)
	err := setEnvironmentVariables()
	if err != nil {
		log.Fatalf("failed to set environment variables: %v", err)
	}
	configFilePtr := flag.String("config", "", "The config file to be provided to the server")
	flag.Parse()
	if *configFilePtr == "" {
		log.Fatalf("Providing a config file is required.")
	}
	cfg, err := config.Validate(*configFilePtr)
	if err != nil {
		log.Fatalf("Couldn't validate config file: %s", err)
	}
	log.Println("config file is valid")
	err = mongodb.TestConnection(cfg.DB.Mongo.Url)
	if err != nil {
		log.Fatalf("failed mongodb test connection: %v", err)
	}
	dbQueries, err := sql.Initialize(cfg.DB.Sql.Path)
	if err != nil {
		log.Fatalf("failed to initialize sql database at %s: %v", cfg.DB.Sql.Path, err)
	}
	log.Println("sql database is initialized")
	err = startNetwork(cfg, dbQueries)
	if err != nil {
		panic(err)
	}
	select {}
}
