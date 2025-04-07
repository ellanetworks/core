package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

type Requester interface {
	// Do performs the HTTP transaction using the provided options.
	Do(ctx context.Context, opts *RequestOptions) (*RequestResponse, error)
}

type RequestType int

const (
	RawRequest RequestType = iota
	SyncRequest
)

type RequestOptions struct {
	Type    RequestType
	Method  string
	Path    string
	Query   url.Values
	Headers map[string]string
	Body    io.Reader
}

type RequestResponse struct {
	StatusCode int
	Headers    http.Header
	// Result can contain request specific JSON data. The result can be
	// unmarshalled into the expected type using the DecodeResult method.
	Result []byte
	// Body is only set for request type RawRequest.
	Body io.ReadCloser
}

// DecodeResult decodes the endpoint-specific result payload that is included as part of
// sync and async request responses. The decoding is performed with the standard JSON
// package, so the usual field tags should be used to prepare the type for decoding.
func (resp *RequestResponse) DecodeResult(result any) error {
	reader := bytes.NewReader(resp.Result)
	dec := json.NewDecoder(reader)
	dec.UseNumber()

	if err := dec.Decode(&result); err != nil {
		return fmt.Errorf("cannot unmarshal: %w", err)
	}

	if dec.More() {
		return fmt.Errorf("cannot unmarshal: cannot parse json value")
	}

	return nil
}

type doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Config allows the user to customize client behavior.
type Config struct {
	// BaseURL contains the base URL where Ella Core is expected to be.
	BaseURL string
}

// A Client knows how to talk to the the Ella Core API.
type Client struct {
	Requester Requester
	host      string
	token     string
}

func (c *Client) GetToken() string {
	return c.token
}

func New(config *Config) (*Client, error) {
	if config == nil {
		config = &Config{}
	}

	client := &Client{}

	requester, err := newDefaultRequester(client, config)
	if err != nil {
		return nil, err
	}

	client.Requester = requester

	client.host = requester.baseURL.Host

	return client, nil
}

// RequestError is returned when there's an error processing the request.
type RequestError struct{ error }

func (e RequestError) Error() string {
	return fmt.Sprintf("cannot build request: %v", e.error)
}

// ConnectionError represents a connection or communication error.
type ConnectionError struct {
	error
}

func (e ConnectionError) Error() string {
	return fmt.Sprintf("cannot communicate with server: %v", e.error)
}

func (e ConnectionError) Unwrap() error {
	return e.error
}

func (rq *defaultRequester) dispatch(ctx context.Context, method, urlpath string, query url.Values, headers map[string]string, body io.Reader) (*http.Response, error) {
	// fake a url to keep http.Client happy
	u := rq.baseURL
	u.Path = path.Join(rq.baseURL.Path, urlpath)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, RequestError{err}
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rsp, err := rq.doer.Do(req)
	if err != nil {
		return nil, ConnectionError{err}
	}

	return rsp, nil
}

var (
	doRetry   = 250 * time.Millisecond
	doTimeout = 5 * time.Second
)

// retry builds in a retry mechanism for GET failures.
func (rq *defaultRequester) retry(ctx context.Context, method, urlpath string, query url.Values, headers map[string]string, body io.Reader) (*http.Response, error) {
	retry := time.NewTicker(doRetry)
	defer retry.Stop()

	timeout := time.After(doTimeout)

	var rsp *http.Response

	var err error

	for {
		rsp, err = rq.dispatch(ctx, method, urlpath, query, headers, body)
		if err == nil || method != "GET" {
			break
		}
		select {
		case <-retry.C:
			continue
		case <-timeout:
		case <-ctx.Done():
		}

		break
	}

	if err != nil {
		return nil, err
	}

	return rsp, nil
}

// Do performs the HTTP request according to the provided options, possibly retrying GET requests
// if appropriate for the status reported by the server.
func (rq *defaultRequester) Do(ctx context.Context, opts *RequestOptions) (*RequestResponse, error) {
	httpResp, err := rq.retry(ctx, opts.Method, opts.Path, opts.Query, opts.Headers, opts.Body)
	if err != nil {
		return nil, err
	}

	// Is the result expecting a caller-managed raw body?
	if opts.Type == RawRequest {
		return &RequestResponse{
			StatusCode: httpResp.StatusCode,
			Headers:    httpResp.Header,
			Body:       httpResp.Body,
		}, nil
	}

	defer httpResp.Body.Close()

	var serverResp response
	if err := decodeInto(httpResp.Body, &serverResp); err != nil {
		return nil, err
	}

	// Deal with error type response
	if err := serverResp.err(); err != nil {
		return nil, err
	}

	// Common response
	return &RequestResponse{
		Headers: httpResp.Header,
		Result:  serverResp.Result,
	}, nil
}

func decodeInto(reader io.Reader, v any) error {
	dec := json.NewDecoder(reader)
	if err := dec.Decode(v); err != nil {
		r := dec.Buffered()

		buf, err1 := io.ReadAll(r)
		if err1 != nil {
			buf = []byte(fmt.Sprintf("error reading buffered response body: %s", err1))
		}

		return fmt.Errorf("cannot decode %q: %w", buf, err)
	}

	return nil
}

// A response produced by the REST API will usually fit in this
// (exceptions are the icons/ endpoints obvs)
type response struct {
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

func (rsp *response) err() error {
	if rsp.Error != "" {
		return fmt.Errorf("server error: %s", rsp.Error)
	}

	return nil
}

type defaultRequester struct {
	baseURL url.URL
	doer    doer
	client  *Client
}

func newDefaultRequester(client *Client, opts *Config) (*defaultRequester, error) {
	if opts == nil {
		opts = &Config{}
	}

	var requester *defaultRequester

	baseURL, err := url.Parse(opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse base URL: %w", err)
	}

	requester = &defaultRequester{baseURL: *baseURL}

	requester.doer = &http.Client{}
	requester.client = client

	return requester, nil
}
