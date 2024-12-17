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
