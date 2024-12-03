package db

import (
	"go.uber.org/zap"
)

func Initialize(url string, name string) error {
	SetLogLevel(zap.DebugLevel)
	err := TestConnection(url)
	if err != nil {
		DbLog.Fatalf("failed to connect to MongoDB: %v", err)
		return err
	}
	ConnectMongo(url, name)
	return nil
}
