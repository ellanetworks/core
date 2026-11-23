// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/config"
)

func TestAdvertisedAPIAddress(t *testing.T) {
	orig := config.GetInterfaceIPFunc
	config.GetInterfaceIPFunc = func(name string, _ config.AddressFamily) (string, error) {
		if name == "eth0" {
			return "10.1.2.3", nil
		}

		return "", errors.New("no such interface")
	}

	t.Cleanup(func() { config.GetInterfaceIPFunc = orig })

	cluster := config.Cluster{Enabled: true, AdvertiseAddress: "core-1.core-headless.ns.svc.cluster.local:7000"}

	tests := []struct {
		name    string
		api     config.APIInterface
		cluster config.Cluster
		want    string
	}{
		{"specific address", config.APIInterface{Address: "10.0.0.1", Port: 5002}, cluster, "http://10.0.0.1:5002"},
		{"tls", config.APIInterface{Address: "10.0.0.1", Port: 5002, TLS: config.TLS{Cert: "c", Key: "k"}}, cluster, "https://10.0.0.1:5002"},
		{"ipv6", config.APIInterface{Address: "fd00::1", Port: 5002}, cluster, "http://[fd00::1]:5002"},
		{"unspecified ipv4 uses cluster host", config.APIInterface{Address: "0.0.0.0", Port: 5002}, cluster, "http://core-1.core-headless.ns.svc.cluster.local:5002"},
		{"unspecified ipv6 uses cluster host", config.APIInterface{Address: "::", Port: 5002}, cluster, "http://core-1.core-headless.ns.svc.cluster.local:5002"},
		{"interface name uses its address", config.APIInterface{Name: "eth0", Port: 5002}, cluster, "http://10.1.2.3:5002"},
		{"unresolvable interface name uses cluster host", config.APIInterface{Name: "missing0", Port: 5002}, cluster, "http://core-1.core-headless.ns.svc.cluster.local:5002"},
		{"unspecified without cluster", config.APIInterface{Address: "0.0.0.0", Port: 5002}, config.Cluster{}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{Cluster: tc.cluster}
			cfg.Interfaces.API = tc.api

			if got := advertisedAPIAddress(cfg); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
