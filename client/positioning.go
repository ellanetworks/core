// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import "context"

type PositioningSession struct {
	ID          string `json:"id"`
	SUPI        string `json:"supi"`
	SessionType int    `json:"session_type"`
	Method      string `json:"method"`
	Status      int    `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// ListPositioningSessions returns the active positioning sessions. The endpoint
// responds with a bare array and is not paginated.
func (c *Client) ListPositioningSessions(ctx context.Context) ([]PositioningSession, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/beta/positioning/sessions",
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
