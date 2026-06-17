// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func GetMetrics() http.Handler {
	return promhttp.Handler()
}
