// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"
	"net/url"
)

type PositioningSession struct {
	ID          string `json:"id"`
	SUPI        string `json:"supi"`
	SessionType int    `json:"session_type"`
	Method      string `json:"method"`
	Status      int    `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// ListPositioningSessions returns the positioning sessions for a subscriber.
// The server requires supi and responds with a bare, non-paginated array.
func (c *Client) ListPositioningSessions(ctx context.Context, supi string) ([]PositioningSession, error) {
	if supi == "" {
		return nil, fmt.Errorf("supi is required")
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/beta/positioning/sessions",
		Query:  url.Values{"supi": {supi}},
	})
	if err != nil {
		return nil, err
	}

	var sessions []PositioningSession

	if err := resp.DecodeResult(&sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}
