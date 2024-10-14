package server

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/yeastengine/ella/internal/db/sql"
)

type HandlerConfig struct {
	DBQueries *sql.Queries
	JWTSecret []byte
}

func generateJWTSecret() ([]byte, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return bytes, fmt.Errorf("failed to generate JWT secret: %w", err)
	}
	return bytes, nil
}

func New(port int, cert []byte, key []byte, dbQueries *sql.Queries) (*http.Server, error) {
	jwtSecret, err := generateJWTSecret()
	if err != nil {
		return nil, err
	}
	env := &HandlerConfig{
		DBQueries: dbQueries,
		JWTSecret: jwtSecret,
	}
	router := NewEllaRouter(env)

	serverCerts, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	s := &http.Server{
		Addr: fmt.Sprintf(":%d", port),

		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		Handler:        router,
		MaxHeaderBytes: 1 << 20,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{serverCerts},
		},
	}

	return s, nil
}

func Start(port int, cert []byte, key []byte, dbQueries *sql.Queries) error {
	srv, err := New(port, cert, key, dbQueries)
	if err != nil {
		return err
	}
	go func() {
		if err := srv.ListenAndServeTLS("", ""); err != nil {
			log.Fatalf("Server failed: %s", err)
		}
	}()
	log.Printf("Server started at https://127.0.0.1:%d", port)
	return nil
}
