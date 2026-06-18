// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func GetMetrics() http.Handler {
	return promhttp.Handler()
}
