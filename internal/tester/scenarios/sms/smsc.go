// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	resultAbsentUser        = 5550
	resultSMDeliveryFailure = 5555
)

type smscClient struct {
	base string
	http *http.Client
}

type smscMessage struct {
	ID       int64  `json:"id"`
	Status   string `json:"status"`
	Attempts []smscAttempt
}

type smscAttempt struct {
	Step              string            `json:"step"`
	NodeType          string            `json:"node_type"`
	Outcome           string            `json:"outcome"`
	ResultCode        *uint32           `json:"result_code"`
	FailureCause      string            `json:"failure_cause"`
	TPFailureCause    string            `json:"tp_failure_cause"`
	AbsentDiagnostic  string            `json:"absent_diagnostic"`
	AbsentDiagnostics map[string]string `json:"absent_diagnostics"`
}

func newSMSC(p *params) (*smscClient, error) {
	if p.SMSCAPI == "" {
		return nil, fmt.Errorf("--smsc-api-address is required")
	}

	return &smscClient{base: strings.TrimSuffix(p.SMSCAPI, "/"), http: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *smscClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}

		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("SMSC %s %s: %s: %s", method, path, resp.Status, raw)
	}

	envelope := struct {
		Result json.RawMessage `json:"result"`
	}{}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("SMSC %s %s: %w", method, path, err)
	}

	return json.Unmarshal(envelope.Result, out)
}

func (c *smscClient) submit(ctx context.Context, to, text string) (int64, error) {
	var out struct {
		Items []smscMessage `json:"items"`
	}

	if err := c.do(ctx, http.MethodPost, "/api/v1/messages", map[string]string{"from": injectedFrom, "to": to, "text": text}, &out); err != nil {
		return 0, err
	}

	if len(out.Items) != 1 {
		return 0, fmt.Errorf("SMSC stored %d messages, want 1", len(out.Items))
	}

	return out.Items[0].ID, nil
}

func (c *smscClient) message(ctx context.Context, id int64) (smscMessage, error) {
	var m smscMessage

	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/messages/%d", id), nil, &m); err != nil {
		return smscMessage{}, err
	}

	var attempts struct {
		Items []smscAttempt `json:"items"`
	}

	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/messages/%d/attempts?per_page=100", id), nil, &attempts); err != nil {
		return smscMessage{}, err
	}

	m.Attempts = attempts.Items

	return m, nil
}

func (c *smscClient) latestTo(ctx context.Context, to string) (smscMessage, error) {
	var out struct {
		Items []smscMessage `json:"items"`
	}

	if err := c.do(ctx, http.MethodGet, "/api/v1/messages?to="+url.QueryEscape(to), nil, &out); err != nil {
		return smscMessage{}, err
	}

	var latest smscMessage

	for _, m := range out.Items {
		if m.ID > latest.ID {
			latest = m
		}
	}

	if latest.ID == 0 {
		return smscMessage{}, fmt.Errorf("SMSC holds no message to %s", to)
	}

	return c.message(ctx, latest.ID)
}

func (c *smscClient) waitFor(ctx context.Context, timeout time.Duration, what string, fetch func(context.Context) (smscMessage, error), done func(smscMessage) bool) error {
	deadline := time.Now().Add(timeout)

	for {
		m, err := fetch(ctx)
		if err == nil && done(m) {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("SMSC: timed out waiting for %s (last %+v, %v)", what, m, err)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (m smscMessage) absentWith(diagnostic string) bool {
	return m.any(func(a smscAttempt) bool {
		if a.ResultCode == nil || *a.ResultCode != resultAbsentUser {
			return false
		}

		if a.AbsentDiagnostic == diagnostic {
			return true
		}

		for _, d := range a.AbsentDiagnostics {
			if d == diagnostic {
				return true
			}
		}

		return false
	})
}

func (m smscMessage) failedWith(cause, tpCause string) bool {
	return m.any(func(a smscAttempt) bool {
		return a.ResultCode != nil && *a.ResultCode == resultSMDeliveryFailure && a.FailureCause == cause && a.TPFailureCause == tpCause
	})
}

func (m smscMessage) deliveredVia(nodeType string) bool {
	return m.Status == "delivered" && m.any(func(a smscAttempt) bool {
		return a.Step == "delivery" && a.Outcome == "success" && a.NodeType == nodeType
	})
}

func (m smscMessage) any(match func(smscAttempt) bool) bool {
	for _, a := range m.Attempts {
		if match(a) {
			return true
		}
	}

	return false
}
