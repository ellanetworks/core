package client

import (
	"context"
	"fmt"
	"sync/atomic"
)

const maxHARetries = 3

// haRequester wraps multiple defaultRequesters and provides round-robin with
// automatic retry on 503 responses (leader unavailable).
type haRequester struct {
	requesters []*defaultRequester
	current    atomic.Uint64
	client     *Client
}

func newHARequester(client *Client, opts *Config) (*haRequester, error) {
	if len(opts.BaseURLs) == 0 {
		return nil, fmt.Errorf("BaseURLs must not be empty for HA mode")
	}

	requesters := make([]*defaultRequester, 0, len(opts.BaseURLs))
	for _, rawURL := range opts.BaseURLs {
		singleOpts := &Config{
			BaseURL:   rawURL,
			APIToken:  opts.APIToken,
			TLSConfig: opts.TLSConfig,
		}

		rq, err := newDefaultRequester(client, singleOpts)
		if err != nil {
			return nil, fmt.Errorf("cannot create requester for %s: %w", rawURL, err)
		}

		requesters = append(requesters, rq)
	}

	return &haRequester{
		requesters: requesters,
		client:     client,
	}, nil
}

func (ha *haRequester) Do(ctx context.Context, opts *RequestOptions) (*RequestResponse, error) {
	n := len(ha.requesters)
	start := int(ha.current.Add(1)-1) % n

	retries := maxHARetries
	if retries > n {
		retries = n
	}

	var lastErr error

	for attempt := range retries {
		idx := (start + attempt) % n
		rq := ha.requesters[idx]

		resp, err := rq.Do(ctx, opts)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 503 {
			lastErr = fmt.Errorf("server %s returned 503", rq.baseURL.String())
			continue
		}

		return resp, nil
	}

	return nil, ConnectionError{fmt.Errorf("all endpoints failed after %d attempts: %w", retries, lastErr)}
}

func (ha *haRequester) host() string {
	idx := int(ha.current.Load()) % len(ha.requesters)
	return ha.requesters[idx].baseURL.Host
}
