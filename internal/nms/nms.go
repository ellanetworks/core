package nms

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nms/server"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func Start(dbInstance *db.Database, port int, cert_file string, key_file string) error {
	router := server.NewHandler(dbInstance)

	go func() {
		httpAddr := ":" + strconv.Itoa(port)
		h2Server := &http2.Server{
			IdleTimeout: 1 * time.Millisecond,
		}
		server := &http.Server{
			Addr:    httpAddr,
			Handler: h2c.NewHandler(router, h2Server),
		}
		err := server.ListenAndServeTLS(cert_file, key_file)
		if err != nil {
			logger.NmsLog.Errorln("couldn't start API server:", err)
		}
	}()
	logger.NmsLog.Infof("API server started on https://localhost:%d", port)
	return nil
}
