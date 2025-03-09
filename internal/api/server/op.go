package server

import (
	"crypto/aes"
	"fmt"
)

func deriveOPc(K []byte, OP []byte) ([]byte, error) {
	// Ensure the key and OP are 16 bytes (128 bits)
	if len(K) != 16 || len(OP) != 16 {
		return nil, fmt.Errorf("k and op must be 16 bytes (128 bits) each")
	}

	// Create AES cipher with K
	block, err := aes.NewCipher(K)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	// Output buffer for OPc
	OPc := make([]byte, 16)

	// Encrypt OP with K to get OPc
	block.Encrypt(OPc, OP)

	return OPc, nil
}
