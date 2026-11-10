// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pki

import (
	"bytes"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	MinSecretSize = 32
	ExporterSize  = 32
	joinKeyInfo   = "ella-core cluster join v1"
	exporterLabel = "EXPORTER-ella-core-cluster-join"
)

const (
	RoleClient = "client"
	RoleServer = "server"
)

var ErrInvalidSecret = errors.New("invalid cluster secret")

func ParseSecrets(raw []byte) ([][]byte, error) {
	var keys [][]byte

	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		secret, err := decodeSecret(line)
		if err != nil {
			return nil, fmt.Errorf("%w: line %d is not base64", ErrInvalidSecret, i+1)
		}

		if len(secret) < MinSecretSize {
			return nil, fmt.Errorf("%w: line %d holds %d bytes, want at least %d", ErrInvalidSecret, i+1, len(secret), MinSecretSize)
		}

		key, err := hkdf.Key(sha256.New, secret, nil, joinKeyInfo, sha256.Size)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidSecret, err)
		}

		keys = append(keys, key)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: the file holds no secret", ErrInvalidSecret)
	}

	return keys, nil
}

func decodeSecret(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}

	return nil, errors.New("not base64")
}

func ReadSecrets(path string) ([][]byte, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- operator-configured cluster.secret-file
	if err != nil {
		return nil, fmt.Errorf("read the cluster secret: %w", err)
	}

	return ParseSecrets(raw)
}

type ProofInput struct {
	Role        string
	Exporter    []byte
	NodeID      string
	Fingerprint string
	Address     string
	BodyHash    []byte
}

func (p ProofInput) encode() []byte {
	var b bytes.Buffer

	for _, field := range [][]byte{
		[]byte(p.Role), p.Exporter, []byte(p.NodeID), []byte(p.Fingerprint), []byte(p.Address), p.BodyHash,
	} {
		_ = binary.Write(&b, binary.BigEndian, uint32(len(field))) // #nosec G115 -- fields are bounded by the request body limit
		b.Write(field)
	}

	return b.Bytes()
}

func Prove(key []byte, in ProofInput) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(in.encode())

	return mac.Sum(nil)
}

func VerifyProof(keys [][]byte, in ProofInput, proof []byte) bool {
	ok := false

	for _, key := range keys {
		if hmac.Equal(Prove(key, in), proof) {
			ok = true
		}
	}

	return ok
}

func Exporter(cs tls.ConnectionState) ([]byte, error) {
	return cs.ExportKeyingMaterial(exporterLabel, nil, ExporterSize)
}

func BodyHash(body []byte) []byte {
	sum := sha256.Sum256(body)

	return sum[:]
}
