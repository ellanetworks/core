package client_test

import (
	"context"

	"github.com/ellanetworks/core/client"
)

type fakeRequester struct {
	response *client.RequestResponse
	err      error
	// lastOpts holds the most recent RequestOptions passed in, so that we can verify
	// that the Login method constructs the request correctly.
	lastOpts *client.RequestOptions
}

func (f *fakeRequester) Do(ctx context.Context, opts *client.RequestOptions) (*client.RequestResponse, error) {
	f.lastOpts = opts
	return f.response, f.err
}
