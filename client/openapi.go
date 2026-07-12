// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"
	"io"
)

// GetOpenAPISpec returns the server's OpenAPI specification as raw YAML.
func (c *Client) GetOpenAPISpec(ctx context.Context) ([]byte, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   RawRequest,
		Method: "GET",
		Path:   "api/v1/openapi.yaml",
	})
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get openapi spec: unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
