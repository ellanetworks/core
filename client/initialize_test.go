// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestInitialize_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "System initialized successfully", "token": "inittoken"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	initializeOpts := &client.InitializeOptions{
		Email:    "user@example.com",
		Password: "secret",
	}

	ctx := context.Background()

	err := clientObj.Initialize(ctx, initializeOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	token := clientObj.GetToken()
	if token != "inittoken" {
		t.Errorf("expected token 'inittoken', got: %s", token)
	}
}
