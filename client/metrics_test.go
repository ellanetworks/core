// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

// TestGetMetricsSuccess verifies that valid Prometheus metrics text is parsed correctly.
func TestGetMetricsSuccess(t *testing.T) {
	metricsText := `
# HELP Some metric description
app_upf_bytes_total{direction="downlink"} 1234
app_upf_bytes_total{direction="uplink"} 5678
app_registration_attempts_total{rat="5g",result="accept",type="Initial registration"} 3
process_open_fds 9
`
	resp := &client.RequestResponse{
		StatusCode: 200,
		Headers:    nil,
		Body:       io.NopCloser(strings.NewReader(metricsText)),
	}
	fake := &fakeRequester{
		response: resp,
		err:      nil,
	}
	// Inject the fake requester into the client.
	c := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	metrics, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if v, ok := metrics[`app_upf_bytes_total{direction="downlink"}`]; !ok || v != 1234 {
		t.Errorf("Expected downlink app_upf_bytes_total to be 1234, got %v", v)
	}

	if v, ok := metrics[`app_upf_bytes_total{direction="uplink"}`]; !ok || v != 5678 {
		t.Errorf("Expected uplink app_upf_bytes_total to be 5678, got %v", v)
	}

	if v, ok := metrics[`app_registration_attempts_total{rat="5g",result="accept",type="Initial registration"}`]; !ok || v != 3 {
		t.Errorf("Expected a label value with a space to parse to 3, got %v", v)
	}

	if v, ok := metrics["process_open_fds"]; !ok || v != 9 {
		t.Errorf("Expected process_open_fds to be 9, got %v", v)
	}
}

// TestGetMetricsRequesterError verifies that an error from the requester is returned.
func TestGetMetricsRequesterError(t *testing.T) {
	fake := &fakeRequester{
		response: nil,
		err:      errors.New("request failed"),
	}
	c := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := c.GetMetrics(ctx)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "request failed" {
		t.Errorf("Expected error 'request failed', got: %v", err)
	}
}

// TestGetMetricsInvalidData verifies that invalid metric data (non-numeric value) returns an error.
func TestGetMetricsInvalidData(t *testing.T) {
	metricsText := `app_upf_bytes_total notanumber`
	resp := &client.RequestResponse{
		StatusCode: 200,
		Headers:    nil,
		Body:       io.NopCloser(strings.NewReader(metricsText)),
	}
	fake := &fakeRequester{
		response: resp,
		err:      nil,
	}
	c := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := c.GetMetrics(ctx)
	if err == nil {
		t.Fatal("Expected error due to invalid metric value, got nil")
	}
}

// TestGetMetricsEmptyResponse verifies that an empty response returns an empty map.
func TestGetMetricsEmptyResponse(t *testing.T) {
	metricsText := ``
	resp := &client.RequestResponse{
		StatusCode: 200,
		Headers:    nil,
		Body:       io.NopCloser(strings.NewReader(metricsText)),
	}
	fake := &fakeRequester{
		response: resp,
		err:      nil,
	}
	c := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	metrics, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("Expected no error for empty response, got: %v", err)
	}

	if len(metrics) != 0 {
		t.Errorf("Expected empty metrics map, got: %v", metrics)
	}
}
