package fixture

import (
	"crypto/ecdh"
	"encoding/hex"
	"strings"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// OperatorSpec captures the singleton operator settings. Because the
// operator is a singleton in Ella Core, subtests that provision different
// values will overwrite each other; the top-level fixture invocation sets
// these once for the compose lifetime via OperatorDefault.
type OperatorSpec struct {
	MCC           string
	MNC           string
	SupportedTACs []string
}

// Operator writes the operator ID + tracking TACs.
func (f *F) Operator(spec OperatorSpec) {
	f.t.Helper()

	if err := f.c.UpdateOperatorID(f.ctx, &client.UpdateOperatorIDOptions{
		Mcc: spec.MCC,
		Mnc: spec.MNC,
	}); err != nil {
		f.fatalf("update operator ID: %v", err)
	}

	if len(spec.SupportedTACs) > 0 {
		if err := f.c.UpdateOperatorTracking(f.ctx, &client.UpdateOperatorTrackingOptions{
			SupportedTacs: spec.SupportedTACs,
		}); err != nil {
			f.fatalf("update operator tracking: %v", err)
		}
	}
}

// OperatorDefault writes scenarios-package defaults to the operator.
func (f *F) OperatorDefault() {
	f.Operator(OperatorSpec{
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SupportedTACs: []string{scenarios.DefaultTAC},
	})
}

// HomeNetworkKeySpec carries a hex-encoded X25519 private key and a
// key identifier. When PrivateKey is empty, a key is auto-generated.
type HomeNetworkKeySpec struct {
	KeyIdentifier int
	Scheme        string
	PrivateKey    string
}

// HomeNetworkKey creates a home network key if the operator does not
// already serve one with the matching public key. Idempotent.
func (f *F) HomeNetworkKey(spec HomeNetworkKeySpec) {
	f.t.Helper()

	if spec.PrivateKey == "" {
		return
	}

	privKeyBytes, err := hex.DecodeString(spec.PrivateKey)
	if err != nil {
		f.fatalf("decode private key: %v", err)
	}

	priv, err := ecdh.X25519().NewPrivateKey(privKeyBytes)
	if err != nil {
		f.fatalf("construct X25519 private key: %v", err)
	}

	pubHex := hex.EncodeToString(priv.PublicKey().Bytes())

	op, err := f.c.GetOperator(f.ctx)
	if err != nil {
		f.fatalf("get operator: %v", err)
	}

	for _, k := range op.HomeNetworkKeys {
		if k.PublicKey == pubHex {
			return
		}
	}

	if err := f.c.CreateHomeNetworkKey(f.ctx, &client.CreateHomeNetworkKeyOptions{
		KeyIdentifier: spec.KeyIdentifier,
		Scheme:        spec.Scheme,
		PrivateKey:    hex.EncodeToString(priv.Bytes()),
	}); err != nil && !isAlreadyExists(err) {
		f.fatalf("create home network key: %v", err)
	}
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "already exists")
}
