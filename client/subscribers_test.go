package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreateSubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Subscriber created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	createSubscriberOpts := &client.CreateSubscriberOptions{
		Imsi:           "001010100000022",
		Key:            "5122250214c33e723a5dd523fc145fc0",
		SequenceNumber: "000000000022",
		PolicyName:     "default",
	}

	ctx := context.Background()

	err := clientObj.CreateSubscriber(ctx, createSubscriberOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateSubscriber_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid IMSI"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	createSubscriberOpts := &client.CreateSubscriberOptions{
		Imsi:           "invalid_imsi",
		Key:            "5122250214c33e723a5dd523fc145fc0",
		SequenceNumber: "000000000022",
		PolicyName:     "default",
	}

	ctx := context.Background()

	err := clientObj.CreateSubscriber(ctx, createSubscriberOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetSubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"imsi": "001010100000022", "policyName": "default"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	imsi := "001010100000022"

	getSubOpts := &client.GetSubscriberOptions{
		ID: imsi,
	}

	ctx := context.Background()

	subscriber, err := clientObj.GetSubscriber(ctx, getSubOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if subscriber.Imsi != imsi {
		t.Fatalf("expected IMSI %s, got %s", imsi, subscriber.Imsi)
	}
}

func TestGetSubscriber_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Subscriber not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	imsi := "non_existent_imsi"

	getSubOpts := &client.GetSubscriberOptions{
		ID: imsi,
	}

	ctx := context.Background()

	_, err := clientObj.GetSubscriber(ctx, getSubOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestDeleteSubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Subscriber deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	imsi := "001010100000022"

	deleteSubOpts := &client.DeleteSubscriberOptions{
		ID: imsi,
	}

	ctx := context.Background()

	err := clientObj.DeleteSubscriber(ctx, deleteSubOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteSubscriber_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Subscriber not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	imsi := "non_existent_imsi"

	deleteSubOpts := &client.DeleteSubscriberOptions{
		ID: imsi,
	}

	ctx := context.Background()

	err := clientObj.DeleteSubscriber(ctx, deleteSubOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListSubscribers_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"imsi": "001010100000022", "policyName": "default"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	subscribers, err := clientObj.ListSubscribers(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(subscribers) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(subscribers))
	}
}

func TestListSubscribers_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Internal server error"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.ListSubscribers(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
