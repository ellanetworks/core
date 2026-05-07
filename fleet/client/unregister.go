// Copyright 2026 Ella Networks

package client

import (
	"context"
	"fmt"
	"net/http"
)

func (fc *Fleet) Unregister(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "POST", fc.url+"/api/v1/cores/unregister", nil)
	if err != nil {
		return fmt.Errorf("creating unregister request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := fc.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending unregister: %w", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if err := checkResponseContentType(res); err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unregister failed (status %d)", res.StatusCode)
	}

	return nil
}
