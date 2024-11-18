package main

import (
	"flag"
	"log"
	"os"

	"github.com/yeastengine/ella/internal/amf"
	"github.com/yeastengine/ella/internal/ausf"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/nssf"
	"github.com/yeastengine/ella/internal/pcf"
	"github.com/yeastengine/ella/internal/smf"
	"github.com/yeastengine/ella/internal/udm"
	"github.com/yeastengine/ella/internal/udr"
	"github.com/yeastengine/ella/internal/upf"
	"github.com/yeastengine/ella/internal/webui"
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

func startNetwork(cfg config.Config) error {
	ausfUrl := "http://127.0.0.1:29509"
	amfUrl := "http://127.0.0.1:29518"
	nssfUrl := "http://127.0.0.1:29531"
	pcfUrl := "http://127.0.0.1:29507"
	smfUrl := "http://127.0.0.1:29502"
	udmUrl := "http://127.0.0.1:29503"
	udrUrl := "http://127.0.0.1:29504"
	webuiUrl, err := webui.Start(cfg.DB.Mongo.Url, cfg.DB.Mongo.Name)
	if err != nil {
		return err
	}
	err = amf.Start(ausfUrl, nssfUrl, pcfUrl, smfUrl, udmUrl, udmUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = ausf.Start(udmUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = pcf.Start(amfUrl, udrUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = udr.Start(cfg.DB.Mongo.Url, cfg.DB.Mongo.Name, webuiUrl, cfg.DB.Sql.Path)
	if err != nil {
		return err
	}
	err = udm.Start(udrUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = nssf.Start(webuiUrl)
	if err != nil {
		return err
	}
	err = smf.Start(amfUrl, pcfUrl, udmUrl, webuiUrl)
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
	err = db.TestConnection(cfg.DB.Mongo.Url)
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	err = startNetwork(cfg)
	if err != nil {
		panic(err)
	}
	select {}
}
