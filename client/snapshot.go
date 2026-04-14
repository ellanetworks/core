package client

import (
	"context"
	"fmt"
	"io"
	"os"
)

// CreateSnapshot triggers a Raft snapshot on the cluster leader.
func (c *Client) CreateSnapshot(ctx context.Context) error {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/cluster/snapshot",
	})
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	_ = resp

	return nil
}

type RestoreSnapshotParams struct {
	Path string
}

// RestoreSnapshot uploads a previously-saved snapshot to the cluster leader,
// which installs it on every node via Raft's user-restore path.
func (c *Client) RestoreSnapshot(ctx context.Context, p *RestoreSnapshotParams) error {
	if p == nil || p.Path == "" {
		return fmt.Errorf("path is required")
	}

	f, err := os.Open(p.Path)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %w", err)
	}

	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat snapshot file: %w", err)
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   RawRequest,
		Method: "POST",
		Path:   "api/v1/cluster/snapshot/restore",
		Headers: map[string]string{
			"Content-Type":   "application/octet-stream",
			"Content-Length": fmt.Sprintf("%d", info.Size()),
		},
		Body: io.Reader(f),
	})
	if err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	return nil
}
