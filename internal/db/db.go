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
	cmd *exec.Cmd
}

func isRunning() bool {
	uri := "mongodb://localhost:27017"

	clientOptions := options.Client().ApplyURI(uri)

	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		return false
	}
	defer client.Disconnect(ctx)
	err = client.Ping(ctx, nil)
	if err != nil {
		return false
	} else {
		return true
	}

}

func waitForMongoDB() error {
	for i := 0; i < 3; i++ {
		if isRunning() {
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

	err = waitForMongoDB()
	if err != nil {
		return nil, fmt.Errorf("failed to wait for mongod: %w", err)
	}

	log.Printf("mongod started")

	return &MongoDB{cmd: cmd}, nil
}

func (m *MongoDB) Stop() {
	if m.cmd != nil && m.cmd.Process != nil {
		err := m.cmd.Process.Kill()
		if err != nil {
			log.Printf("failed to kill mongod process: %v", err)
		}
	}
}
