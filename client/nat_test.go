// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetNATInfo_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"enabled": true}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	natInfo, err := clientObj.GetNATInfo(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if natInfo.Enabled != true {
		t.Errorf("expected NAT enabled to be true, got: %v", natInfo.Enabled)
	}
}

func TestUpdateNATInfo_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "NAT Info updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateNATInfoOpts := &client.UpdateNATInfoOptions{
		Enabled: false,
	}

	ctx := context.Background()

	err := clientObj.UpdateNATInfo(ctx, updateNATInfoOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
