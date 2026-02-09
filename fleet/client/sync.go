// Copyright 2026 Ella Networks

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type SyncParams struct {
	Version string `json:"version"`
}

func (fc *Fleet) Sync(ctx context.Context, params *SyncParams) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fc.url+"/api/v1/cores/sync", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating sync request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := fc.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending sync: %w", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
			return fmt.Errorf("sync: unexpected status code %d and failed to decode error: %w", res.StatusCode, err)
		}

		return fmt.Errorf("sync failed (status %d): %s", res.StatusCode, errResp.Error)
	}

	return nil
}
