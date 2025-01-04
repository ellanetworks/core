// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package httpwrapper

import (
	"net/http"
	"net/url"
)

type Request struct {
	Params map[string]string
	Header http.Header
	Query  url.Values
	Body   interface{}
	URL    *url.URL
}

type Response struct {
	Header http.Header
	Status int
	Body   interface{}
}

func NewResponse(code int, h http.Header, body interface{}) *Response {
	ret := &Response{}
	ret.Status = code
	ret.Header = h
	ret.Body = body
	return ret
}
