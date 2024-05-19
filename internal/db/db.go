package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Cmd *exec.Cmd
	URL string
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
			log.Printf("failed to disconnect from mongo: %v", err)
		}
	}()
	err = client.Ping(ctx, nil)
	if err != nil {
		return false
	} else {
		return true
	}
}

func (m *MongoDB) waitForStartup() error {
	for i := 0; i < 3; i++ {
		if m.isRunning() {
			return nil
		}
		log.Printf("waiting for mongod to start")
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("mongod did not start")
}

func StartMongoDB(dbPath string) (*MongoDB, error) {
	cmd := exec.Command("/snap/moose/current/usr/bin/mongod", "--dbpath", dbPath)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start mongod: %w", err)
	}
	mongo := &MongoDB{
		Cmd: cmd,
		URL: "mongodb://localhost:27017",
	}
	err = mongo.waitForStartup()
	if err != nil {
		return nil, fmt.Errorf("failed to wait for mongod: %w", err)
	}
	log.Printf("mongod started")
	return mongo, nil
}

func (m *MongoDB) Stop() {
	if m.Cmd != nil && m.Cmd.Process != nil {
		err := m.Cmd.Process.Kill()
		if err != nil {
			log.Printf("failed to kill mongod process: %v", err)
		}
	}
}
