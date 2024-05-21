package db

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	MongoBinariesPath = "/snap/moose/current/usr/bin"
	LogPrefix         = "mongodb: "
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

func (m *MongoDB) waitForStartup() error {
	for i := 0; i < 3; i++ {
		if m.isRunning() {
			return nil
		}
		AppLog.Printf("mongod not started yet, retrying...")
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("mongod did not start")
}

func StartMongoDB(dbPath string) (*MongoDB, error) {
	nullFile, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/null: %w", err)
	}
	defer nullFile.Close()
	cmd := exec.Command(MongoBinariesPath+"/mongod", "--dbpath", dbPath, "--replSet", "rs0", "--bind_ip", "127.0.0.1")

	cmd.Stdout = nullFile
	cmd.Stderr = nullFile

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start mongod: %w", err)
	}
	mongo := &MongoDB{
		Cmd: cmd,
		URL: "mongodb://localhost:27017",
	}

	err = initializeReplicaSetWithRetry()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize replica set: %w", err)
	}

	err = mongo.waitForStartup()
	if err != nil {
		return nil, fmt.Errorf("failed to wait for mongod: %w", err)
	}

	AppLog.Printf("mongod started with replica set")
	return mongo, nil
}

func initializeReplicaSetWithRetry() error {
	for i := 0; i < 10; i++ {
		err := initializeReplicaSet()
		if err == nil {
			return nil
		}
		AppLog.Printf("failed to initialize replica set, retrying...")
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("failed to initialize replica set after multiple attempts")
}

func initializeReplicaSet() error {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(MongoBinariesPath+"/mongosh", "--eval", "rs.initiate()")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if bytes.Contains(stderr.Bytes(), []byte("already initialized")) {
			AppLog.Printf("replica set already initialized")
			return nil
		}
		return fmt.Errorf("failed to run rs.initiate(): %w: %s", err, stderr.String())
	}

	AppLog.Printf("replica set initialized successfully: %s", stdout.String())
	return nil
}

func (m *MongoDB) Stop() {
	if m.Cmd != nil && m.Cmd.Process != nil {
		err := m.Cmd.Process.Kill()
		if err != nil {
			AppLog.Printf("failed to kill mongod process: %v", err)
		}
	}
}
