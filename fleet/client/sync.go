// Copyright 2026 Ella Networks

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type SubscriberUsageEntry struct {
	EpochDay      int64  `json:"epoch_day"`
	IMSI          string `json:"imsi"`
	UplinkBytes   int64  `json:"uplink_bytes"`
	DownlinkBytes int64  `json:"downlink_bytes"`
}

// FlowEntry represents a single completed UE data flow captured by the eBPF
// data plane and forwarded to Fleet as part of the sync cycle.
type FlowEntry struct {
	SubscriberID    string `json:"subscriber_id"`
	SourceIP        string `json:"source_ip"`
	DestinationIP   string `json:"destination_ip"`
	SourcePort      int    `json:"source_port"`
	DestinationPort int    `json:"destination_port"`
	Protocol        int    `json:"protocol"`
	Packets         int64  `json:"packets"`
	Bytes           int64  `json:"bytes"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	Direction       string `json:"direction"`
	// Action mirrors dbwriter.FlowReport.Action: "allow" when the flow
	// matched an allow rule, "drop" when it was dropped by an SDF
	// filter. Empty when the producer didn't classify (older Cores).
	Action string `json:"action,omitempty"`
}

type SyncParams struct {
	Version           string                 `json:"version"`
	ClusterEnabled    bool                   `json:"cluster_enabled"`
	NodeID            int                    `json:"node_id,omitempty"`
	ClusterID         string                 `json:"cluster_id,omitempty"`
	IsLeader          bool                   `json:"is_leader"`
	LastKnownRevision int64                  `json:"last_known_revision"`
	Status            *EllaCoreStatus        `json:"status,omitempty"`
	Metrics           EllaCoreMetrics        `json:"metrics"`
	Flows             []FlowEntry            `json:"flows,omitempty"`
	SubscriberUsage   []SubscriberUsageEntry `json:"subscriber_usage,omitempty"`
}

type SyncResponse struct {
	Config         *Config `json:"config,omitempty"`
	ConfigRevision int64   `json:"config_revision"`
}

func (fc *Fleet) Sync(ctx context.Context, params *SyncParams) (*SyncResponse, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fc.url+"/api/v1/cores/sync", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating sync request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	fc.addAuth(req)

	res, err := fc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending sync: %w", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if err := checkResponseContentType(res); err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("sync: unexpected status code %d and failed to decode error: %w", res.StatusCode, err)
		}

		return nil, fmt.Errorf("sync failed (status %d): %s", res.StatusCode, errResp.Error)
	}

	var envelope Response
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decoding response envelope: %w", err)
	}

	var syncResponse SyncResponse
	if err := json.Unmarshal(envelope.Result, &syncResponse); err != nil {
		return nil, fmt.Errorf("decoding sync result: %w", err)
	}

	return &syncResponse, nil
}
