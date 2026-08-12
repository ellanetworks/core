// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/ngap"
)

func decodeB64(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}

	return nil, fmt.Errorf("not valid base64")
}

// The rendered label is the codec's, for every id and procedure code, so a
// local table reintroduced anywhere in the render path fails here.
func TestLabelsComeFromTheCodec(t *testing.T) {
	for id := 0; id <= 0xFFFF; id++ {
		name, known := ngap.ProtocolIEIDName(ngap.ProtocolIEID(id))

		got := ieEnum(ngap.ProtocolIEID(id))
		if got.Label != name || got.Unknown == known {
			t.Fatalf("IE id %d renders %+v, want label %q known %v", id, got, name, known)
		}
	}

	for code := 0; code <= 0xFF; code++ {
		name, known := ngap.ProcedureCodeName(ngap.ProcedureCode(code))

		got := procedureCodeToEnum(ngap.ProcedureCode(code))
		if got.Label != name || got.Unknown == known {
			t.Fatalf("procedure code %d renders %+v, want label %q known %v", code, got, name, known)
		}
	}
}
