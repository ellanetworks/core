package client

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

type CreateBackupParams struct {
	Path string
}

type RestoreBackupParams struct {
	Path string
}

// CreateBackup creates a backup and saves it to the specified path.
func (c *Client) CreateBackup(ctx context.Context, p *CreateBackupParams) error {
	if p == nil {
		return fmt.Errorf("CreateBackupParams is nil")
	}

	if p.Path == "" {
		return fmt.Errorf("path is required")
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   RawRequest,
		Method: "POST",
		Path:   "api/v1/backup",
	})
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	out, err := os.Create(p.Path)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}

	defer func() {
		_ = out.Close()
	}()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write backup to file: %w", err)
	}

	return nil
}

func (c *Client) RestoreBackup(ctx context.Context, p *RestoreBackupParams) error {
	if p == nil || p.Path == "" {
		return fmt.Errorf("path is required")
	}

	f, err := os.Open(p.Path)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	// Stream multipart to the pipe.
	go func() {
		defer func() {
			_ = pw.Close()
		}()

		part, err := mw.CreateFormFile("backup", filepath.Base(p.Path)) // must match r.FormFile("backup")
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		if _, err := io.Copy(part, f); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		if err := mw.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   RawRequest,
		Method: "POST",
		Path:   "api/v1/restore",
		Headers: map[string]string{
			"Content-Type": mw.FormDataContentType(),
		},
		Body: pr,
	})
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	return nil
}
