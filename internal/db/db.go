package db

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Cmd *exec.Cmd
	URL string
}

var (
	log    *logrus.Logger
	AppLog *logrus.Entry
)

func init() {
	log = logrus.New()
	log.Formatter = &formatter.Formatter{
		TimestampFormat: time.RFC3339,
		TrimMessages:    true,
		NoFieldsSpace:   true,
		HideKeys:        true,
		FieldsOrder:     []string{"component", "category"},
	}
	AppLog = log.WithFields(logrus.Fields{"component": "Database", "category": "App"})
}

func (m *MongoDB) getClient() (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(m.URL)
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create mongo client: %w", err)
	}
	return client, nil
}

func (m *MongoDB) isRunning() bool {
	client, err := m.getClient()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		return false
	}
	defer func() {
		err = client.Disconnect(ctx)
		if err != nil {
			AppLog.Printf("failed to disconnect from mongo: %v", err)
		}
	}()
	err = client.Ping(ctx, nil)
	if err != nil {
		return false
	} else {
		return true
	}
}
