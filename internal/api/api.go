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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// interfaceKernelMap maps the interface string to the kernel.NetworkInterface enum.
var interfaceDBKernelMap = map[db.NetworkInterface]kernel.NetworkInterface{
	db.N3: kernel.N3,
	db.N6: kernel.N6,
}

func Start(dbInstance *db.Database, port int, certFile string, keyFile string, n3Interface string, n6Interface string) error {
	jwtSecret, err := server.GenerateJWTSecret()
	if err != nil {
		return fmt.Errorf("couldn't generate jwt secret: %v", err)
	}
	kernelInt := kernel.NewRealKernel(n3Interface, n6Interface)
	router := server.NewHandler(dbInstance, kernelInt, jwtSecret)

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
			logger.APILog.Errorln("couldn't start API server:", err)
		}
	}()
	logger.APILog.Infof("API server started on https://localhost:%d", port)

	// Reconcile routes on startup and every 5 minutes
	go func() {
		for {
			err := ReconcileRoutes(dbInstance, kernelInt)
			if err != nil {
				logger.APILog.Errorf("couldn't reconcile routes: %v", err)
			}
			time.Sleep(5 * time.Minute)
		}
	}()
	return nil
}

func ReconcileRoutes(dbInstance *db.Database, kernelInt kernel.Kernel) error {
	expectedRoutes, err := dbInstance.ListRoutes()
	if err != nil {
		return fmt.Errorf("couldn't list routes: %v", err)
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
	logger.APILog.Infoln("routes reconciled")
	return nil
}
