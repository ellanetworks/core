package client

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

const maxHARetries = 3

const haRetryBackoff = 100 * time.Millisecond

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

			if attempt < retries-1 {
				if waitErr := waitForRetry(ctx, time.Duration(attempt+1)*haRetryBackoff); waitErr != nil {
					return nil, ConnectionError{waitErr}
				}
			}

			continue
		}

		if resp.StatusCode == 503 {
			lastErr = fmt.Errorf("server %s returned 503", rq.baseURL.String())

			if attempt < retries-1 {
				if waitErr := waitForRetry(ctx, time.Duration(attempt+1)*haRetryBackoff); waitErr != nil {
					return nil, ConnectionError{waitErr}
				}
			}

			continue
		}

		return resp, nil
	}

	return nil, ConnectionError{fmt.Errorf("all endpoints failed after %d attempts: %w", retries, lastErr)}
}

func waitForRetry(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// host returns the host of the currently selected requester. The result may
// change after each Do call due to round-robin rotation; callers that need a
// stable value should capture it once (as client.New does at construction).
func (ha *haRequester) host() string {
	idx := int(ha.current.Load()) % len(ha.requesters)
	return ha.requesters[idx].baseURL.Host
}
