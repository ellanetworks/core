// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"net/http"
)

type Response struct {
	Header http.Header
	Status int
	Body   interface{}
}
