package ngap_test

import (
	"encoding/base64"
	"fmt"
)

func decodeB64(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}

	return nil, fmt.Errorf("not valid base64")
}

func encodeB64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
