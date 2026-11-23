// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"net"
	"strconv"

	"github.com/ellanetworks/core/internal/config"
)

func advertisedAPIAddress(cfg config.Config) string {
	api := cfg.Interfaces.API

	scheme := "http"
	if api.TLS.Cert != "" && api.TLS.Key != "" {
		scheme = "https"
	}

	host := api.Address

	if host == "" && api.Name != "" {
		if ip, err := config.GetInterfaceIP(api.Name, config.AnyFamily); err == nil {
			host = ip
		}
	}

	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = ""

		if cfg.Cluster.Enabled {
			if h, _, err := net.SplitHostPort(cfg.Cluster.AdvertiseAddress); err == nil {
				host = h
			}
		}
	}

	if host == "" {
		return ""
	}

	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(api.Port))
}
