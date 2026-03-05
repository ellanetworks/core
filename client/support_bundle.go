package client

import (
	"context"
	"fmt"
	"io"
	"os"
)

type GenerateSupportBundleParams struct {
	// Path is the destination file path where the downloaded bundle will be saved.
	Path string
}

// GenerateSupportBundle requests a support bundle from the server and saves
// the returned gzipped tar to the provided path.
func (c *Client) GenerateSupportBundle(ctx context.Context, p *GenerateSupportBundleParams) error {
	if p == nil || p.Path == "" {
		return fmt.Errorf("path is required")
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   RawRequest,
		Method: "POST",
		Path:   "api/v1/support-bundle",
	})
	if err != nil {
		return fmt.Errorf("failed to generate support bundle: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	out, err := os.Create(p.Path)
	if err != nil {
		return fmt.Errorf("failed to create support bundle file: %w", err)
	}

	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write support bundle to file: %w", err)
	}

	return nil
}
