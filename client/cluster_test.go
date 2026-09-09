// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListClusterMembers_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"nodeId":1,"raftAddress":"10.0.0.1:7000","apiAddress":"https://10.0.0.1:5002","binaryVersion":"v1.0.0","suffrage":"voter"},{"nodeId":2,"raftAddress":"10.0.0.2:7000","apiAddress":"https://10.0.0.2:5002","binaryVersion":"v1.0.0","suffrage":"voter"}]`),
		},
	}
	c := &client.Client{Requester: fake}

	members, err := c.ListClusterMembers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	if members[0].NodeID != 1 {
		t.Errorf("expected nodeId 1, got %d", members[0].NodeID)
	}

	if members[0].RaftAddress != "10.0.0.1:7000" {
		t.Errorf("expected raft address 10.0.0.1:7000, got %s", members[0].RaftAddress)
	}

	if members[0].Suffrage != "voter" {
		t.Errorf("expected suffrage voter, got %s", members[0].Suffrage)
	}

	if members[1].NodeID != 2 {
		t.Errorf("expected nodeId 2, got %d", members[1].NodeID)
	}
}

func TestDrainClusterMember_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"drainState":"draining"}`),
		},
	}
	c := &client.Client{Requester: fake}

	resp, err := c.DrainClusterMember(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.DrainState != "draining" {
		t.Errorf("expected drainState draining, got %s", resp.DrainState)
	}

	if fake.lastOpts.Method != "POST" {
		t.Errorf("expected POST, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/cluster/members/3/drain" {
		t.Errorf("expected api/v1/cluster/members/3/drain, got %s", fake.lastOpts.Path)
	}
}

func TestResumeClusterMember_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message":"Cluster member resumed"}`),
		},
	}
	c := &client.Client{Requester: fake}

	if err := c.ResumeClusterMember(context.Background(), 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Errorf("expected POST, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/cluster/members/3/resume" {
		t.Errorf("expected api/v1/cluster/members/3/resume, got %s", fake.lastOpts.Path)
	}
}

func TestPromoteClusterMember_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message":"Cluster member promoted to voter"}`),
		},
	}
	c := &client.Client{Requester: fake}

	err := c.PromoteClusterMember(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Errorf("expected POST, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/cluster/members/3/promote" {
		t.Errorf("expected api/v1/cluster/members/3/promote, got %s", fake.lastOpts.Path)
	}
}

func TestRemoveClusterMember_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message":"Cluster member removed"}`),
		},
	}
	c := &client.Client{Requester: fake}

	err := c.RemoveClusterMember(context.Background(), 2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" {
		t.Errorf("expected DELETE, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/cluster/members/2" {
		t.Errorf("expected api/v1/cluster/members/2, got %s", fake.lastOpts.Path)
	}
}

func TestRemoveClusterMember_Force(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message":"Cluster member removed"}`),
		},
	}
	c := &client.Client{Requester: fake}

	if err := c.RemoveClusterMember(context.Background(), 2, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.lastOpts.Path != "api/v1/cluster/members/2" {
		t.Errorf("expected a clean path, got %s", fake.lastOpts.Path)
	}

	if got := fake.lastOpts.Query.Get("force"); got != "true" {
		t.Errorf("expected force=true in the query, got %q", got)
	}
}

func TestMintClusterJoinToken_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"token":"AQAA","expiresAt":1714233600}`),
		},
	}
	c := &client.Client{Requester: fake}

	resp, err := c.MintClusterJoinToken(context.Background(), &client.MintJoinTokenOptions{
		NodeID:     2,
		TTLSeconds: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Token != "AQAA" {
		t.Errorf("token = %q", resp.Token)
	}

	if fake.lastOpts.Path != "api/v1/cluster/pki/join-tokens" {
		t.Errorf("path = %q", fake.lastOpts.Path)
	}
}

func TestMintClusterJoinToken_NilOpts(t *testing.T) {
	c := &client.Client{Requester: &fakeRequester{}}

	if _, err := c.MintClusterJoinToken(context.Background(), nil); err == nil {
		t.Fatal("expected error on nil opts")
	}
}
