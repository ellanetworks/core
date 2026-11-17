// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

const LoginLifetime = 10 * time.Minute

var ErrLogin = errors.New("invalid or expired login")

type Login struct {
	State     string `json:"s"`
	Nonce     string `json:"n"`
	Verifier  string `json:"v"`
	Issuer    string `json:"i"`
	ClientID  string `json:"c"`
	ExpiresAt int64  `json:"e"`
}

func (c *Client) NewLogin(now time.Time) Login {
	return Login{
		State:     rand.Text(),
		Nonce:     rand.Text(),
		Verifier:  oauth2.GenerateVerifier(),
		Issuer:    c.settings.Issuer,
		ClientID:  c.settings.ClientID,
		ExpiresAt: now.Add(LoginLifetime).Unix(),
	}
}

func (c *Client) Owns(l Login) bool {
	return l.Issuer == c.settings.Issuer && l.ClientID == c.settings.ClientID
}

func Seal(key []byte, name string, l Login) (string, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}

	plain, err := json.Marshal(l)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())
	_, _ = rand.Read(nonce)

	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, plain, []byte(name))), nil
}

func Open(key []byte, name, value string, now time.Time) (Login, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return Login{}, err
	}

	sealed, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(sealed) < aead.NonceSize() {
		return Login{}, ErrLogin
	}

	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(name))
	if err != nil {
		return Login{}, ErrLogin
	}

	var l Login
	if err := json.Unmarshal(plain, &l); err != nil || !now.Before(time.Unix(l.ExpiresAt, 0)) || l.State == "" {
		return Login{}, ErrLogin
	}

	return l, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("login cookie key: %w", err)
	}

	return cipher.NewGCM(block)
}
