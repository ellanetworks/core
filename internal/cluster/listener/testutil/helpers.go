// Copyright 2026 Ella Networks

package testutil

import (
	"crypto/rand"
	"crypto/x509/pkix"
	"io"
	"math/big"
)

// bigIntAlias is a type alias so our public API in pki.go doesn't need
// to import math/big.
type bigIntAlias = big.Int

// small helpers, kept in a separate file so the public surface in pki.go
// doesn't get noisy.

func commonName(cn string) pkix.Name {
	return pkix.Name{CommonName: cn}
}

func bigIntFrom(n int64) *big.Int {
	return big.NewInt(n)
}

func randReader() io.Reader { return rand.Reader }
