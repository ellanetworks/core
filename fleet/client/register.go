// Copyright 2026 Ella Networks

package client

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
)

type RegisterParams struct {
	ActivationToken string `json:"activation_token"`
	PublicKey       string `json:"public_key"`
}

type RegisterResponse struct {
	Certificate   string `json:"certificate"`
	CACertificate string `json:"ca_certificate"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Response struct {
	Result any `json:"result"`
}

func (fc *Fleet) Register(ctx context.Context, activationToken string, publicKey ecdsa.PublicKey) (*RegisterResponse, error) {
	pubKeyPEM, err := marshalPublicKey(&publicKey)
	if err != nil {
		return nil, fmt.Errorf("couldn't marshal public key: %w", err)
	}

	params := &RegisterParams{
		ActivationToken: activationToken,
		PublicKey:       pubKeyPEM,
	}

	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fc.url+"/api/v1/cores/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := fc.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("unexpected status code %d and failed to decode error: %w", res.StatusCode, err)
		}

		return nil, fmt.Errorf("register failed (status %d): %s", res.StatusCode, errResp.Error)
	}

	var envelope Response
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decoding response envelope: %w", err)
	}

	resultBytes, err := json.Marshal(envelope.Result)
	if err != nil {
		return nil, fmt.Errorf("re-marshalling result: %w", err)
	}

	var registerResponse RegisterResponse
	if err := json.Unmarshal(resultBytes, &registerResponse); err != nil {
		return nil, fmt.Errorf("decoding register result: %w", err)
	}

	return &registerResponse, nil
}

func marshalPublicKey(pub *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("marshalling public key: %w", err)
	}

	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}

	return string(pem.EncodeToMemory(block)), nil
}
