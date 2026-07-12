// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetOpenAPISpec_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Body:       io.NopCloser(strings.NewReader("openapi: 3.0.0\ninfo:\n  title: Ella Core\n")),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	spec, err := clientObj.GetOpenAPISpec(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(string(spec), "openapi:") {
		t.Fatalf("expected openapi spec, got: %q", string(spec))
	}
}

func TestGetOpenAPISpec_UnexpectedStatus(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	if _, err := clientObj.GetOpenAPISpec(context.Background()); err == nil {
		t.Fatalf("expected error on 500 status, got none")
	}
}
