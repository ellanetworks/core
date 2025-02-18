package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func Start(dbInstance *db.Database, port int, certFile string, keyFile string) error {
	jwtSecret, err := server.GenerateJWTSecret()
	if err != nil {
		return fmt.Errorf("couldn't generate jwt secret: %v", err)
	}
	router := server.NewHandler(dbInstance, jwtSecret)

	go func() {
		httpAddr := ":" + strconv.Itoa(port)
		h2Server := &http2.Server{
			IdleTimeout: 1 * time.Millisecond,
		}
		server := &http.Server{
			Addr:              httpAddr,
			ReadHeaderTimeout: 5 * time.Second,
			Handler:           h2c.NewHandler(router, h2Server),
		}
		err := server.ListenAndServeTLS(certFile, keyFile)
		if err != nil {
			logger.NmsLog.Errorln("couldn't start API server:", err)
		}
	}()
	logger.NmsLog.Infof("API server started on https://localhost:%d", port)
	return nil
}
