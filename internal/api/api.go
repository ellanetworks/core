package api

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// interfaceDBKernelMap maps the interface string to the kernel.NetworkInterface enum.
var interfaceDBKernelMap = map[db.NetworkInterface]kernel.NetworkInterface{
	db.N3: kernel.N3,
	db.N6: kernel.N6,
}

type Scheme string

const (
	HTTP  Scheme = "http"
	HTTPS Scheme = "https"
)

// routeReconciler is used to reconcile routes periodically.
// In tests we can override it to disable actual reconciliation.
var routeReconciler = ReconcileKernelRouting

func Start(dbInstance *db.Database, port int, scheme Scheme, certFile string, keyFile string, n3Interface string, n6Interface string) error {
	jwtSecret, err := server.GenerateJWTSecret()
	if err != nil {
		return fmt.Errorf("couldn't generate jwt secret: %v", err)
	}
	kernelInt := kernel.NewRealKernel(n3Interface, n6Interface)
	router := server.NewHandler(dbInstance, kernelInt, jwtSecret, gin.ReleaseMode)

	// Start the HTTP server in a goroutine.
	go func() {
		httpAddr := ":" + strconv.Itoa(port)
		h2Server := &http2.Server{
			IdleTimeout: 1 * time.Millisecond,
		}
		srv := &http.Server{
			Addr:              httpAddr,
			ReadHeaderTimeout: 5 * time.Second,
			Handler:           h2c.NewHandler(router, h2Server),
		}
		if scheme == HTTPS {
			if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil {
				logger.APILog.Errorf("couldn't start API server: %v", err)
			}
		} else {
			if err := srv.ListenAndServe(); err != nil {
				logger.APILog.Errorf("couldn't start API server: %v", err)
			}
		}
	}()

	logger.APILog.Infof("API server started on %s://127.0.0.1:%d", scheme, port)

	// Reconcile routes on startup and every 5 minutes.
	go func() {
		for {
			err := routeReconciler(dbInstance, kernelInt)
			if err != nil {
				logger.APILog.Errorf("couldn't reconcile routes: %v", err)
			}
			time.Sleep(5 * time.Minute)
		}
	}()
	return nil
}

func ReconcileKernelRouting(dbInstance *db.Database, kernelInt kernel.Kernel) error {
	expectedRoutes, err := dbInstance.ListRoutes()
	if err != nil {
		return fmt.Errorf("couldn't list routes: %v", err)
	}
	ipForwardingEnabled, err := kernelInt.IsIPForwardingEnabled()
	if err != nil {
		return fmt.Errorf("couldn't check if IP forwarding is enabled: %v", err)
	}
	if !ipForwardingEnabled {
		err := kernelInt.EnableIPForwarding()
		if err != nil {
			return fmt.Errorf("couldn't enable IP forwarding: %v", err)
		}
	}
	for _, route := range expectedRoutes {
		_, ipNetwork, err := net.ParseCIDR(route.Destination)
		if err != nil {
			return fmt.Errorf("couldn't parse destination: %v", err)
		}
		ipGateway := net.ParseIP(route.Gateway)
		if ipGateway == nil || ipGateway.To4() == nil {
			return fmt.Errorf("invalid gateway: %v", route.Gateway)
		}
		ipGateway = ipGateway.To4()
		kernelNetworkInterface, ok := interfaceDBKernelMap[route.Interface]
		if !ok {
			return fmt.Errorf("invalid interface: %v", route.Interface)
		}
		routeExists, err := kernelInt.RouteExists(ipNetwork, ipGateway, route.Metric, kernelNetworkInterface)
		if err != nil {
			return fmt.Errorf("couldn't check if route exists: %v", err)
		}
		if !routeExists {
			err := kernelInt.CreateRoute(ipNetwork, ipGateway, route.Metric, kernelNetworkInterface)
			if err != nil {
				return fmt.Errorf("couldn't create route: %v", err)
			}
		}
	}
	logger.APILog.Debugln("Routes reconciled")
	return nil
}
