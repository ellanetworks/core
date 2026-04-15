package client_test

import (
	"context"
	"errors"
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

func TestListClusterMembers_Failure(t *testing.T) {
	fake := &fakeRequester{
		err: errors.New("connection refused"),
	}
	c := &client.Client{Requester: fake}

	_, err := c.ListClusterMembers(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDrainNode_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message":"draining","transferredLeadership":true,"ransNotified":2,"bgpStopped":true}`),
		},
	}
	c := &client.Client{Requester: fake}

	resp, err := c.DrainNode(context.Background(), &client.DrainOptions{TimeoutSeconds: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.TransferredLeadership {
		t.Error("expected transferredLeadership true")
	}

	if resp.RANsNotified != 2 {
		t.Errorf("expected 2 RANs notified, got %d", resp.RANsNotified)
	}

	if !resp.BGPStopped {
		t.Error("expected bgpStopped true")
	}

	if fake.lastOpts.Method != "POST" {
		t.Errorf("expected POST, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/cluster/drain" {
		t.Errorf("expected api/v1/cluster/drain, got %s", fake.lastOpts.Path)
	}
}

func TestDrainNode_NilOpts(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message":"draining","transferredLeadership":false,"ransNotified":0,"bgpStopped":false}`),
		},
	}
	c := &client.Client{Requester: fake}

	resp, err := c.DrainNode(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message != "draining" {
		t.Errorf("expected message 'draining', got %s", resp.Message)
	}
}

func TestDrainNode_Failure(t *testing.T) {
	fake := &fakeRequester{
		err: errors.New("leader unavailable"),
	}
	c := &client.Client{Requester: fake}

	_, err := c.DrainNode(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
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

func TestPromoteClusterMember_Failure(t *testing.T) {
	fake := &fakeRequester{
		err: errors.New("not found"),
	}
	c := &client.Client{Requester: fake}

	err := c.PromoteClusterMember(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error, got nil")
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

	err := c.RemoveClusterMember(context.Background(), 2)
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

func TestRemoveClusterMember_Failure(t *testing.T) {
	fake := &fakeRequester{
		err: errors.New("server error"),
	}
	c := &client.Client{Requester: fake}

	err := c.RemoveClusterMember(context.Background(), 2)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
