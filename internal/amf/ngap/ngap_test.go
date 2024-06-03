// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
package ngap_test

import (
	"log"
	"testing"
	"time"

	"github.com/yeastengine/canard/internal/amf/context"
	"github.com/yeastengine/canard/internal/amf/factory"
	"github.com/yeastengine/canard/internal/amf/ngap"
	ngaputil "github.com/yeastengine/canard/internal/amf/ngap/util"
	"github.com/yeastengine/canard/internal/amf/util"
)

func init() {
	// Initializing AMF Context from config.
	testAmfConfig := "../amfTest/amfcfg.yaml"
	if err := factory.InitConfigFactory(testAmfConfig); err != nil {
		log.Fatal("Failed to initialzie Factory Config")
	}

	util.InitAmfContext(context.AMF_Self())
}

// TestHandleNGSetupRequest validates package ngap's handling for NGSetupRequest
func TestHandleNGSetupRequest(t *testing.T) {
	// test cases
	testTable := []struct {
		gnbName, tac string
		gnbId        []byte
		bitLength    uint64
		want, testId byte
	}{
		// expecting SuccessfulOutcome
		{
			testId:    1,
			gnbName:   "GNB2",
			tac:       "\x00\x00\x01",
			gnbId:     []byte{0x00, 0x00, 0x08},
			bitLength: 22,
			want:      ngaputil.NgapPDUSuccessfulOutcome,
		},
		// expecting UnsuccessfulOutcome due to unsupported TA
		{
			testId:    2,
			gnbName:   "GNB2",
			tac:       "\x00\x00\x04",
			gnbId:     []byte{0x00, 0x00, 0x08},
			bitLength: 22,
			want:      ngaputil.NgapPDUUnSuccessfulOutcome,
		},
	}

	conn := &ngaputil.TestConn{}
	for _, test := range testTable {
		testNGSetupReq, err := ngaputil.GetNGSetupRequest(test.gnbId, test.bitLength, test.gnbName, test.tac)
		if err != nil {
			t.Log("Failed to to create NGSetupRequest")
			return
		}
		ngap.Dispatch(conn, testNGSetupReq)
		time.Sleep(2 * time.Second)
		// conn.data holds the NGAP response message
		if len(conn.Data) == 0 {
			t.Error("Unexpected message drop")
			return
		}

		// The first byte of the NGAPPDU indicates the type of NGAP Message
		if conn.Data[0] != test.want {
			t.Error("Test case", test.testId, "failed.  Want:",
				ngaputil.MessageTypeMap[test.want], ",  Got:", ngaputil.MessageTypeMap[conn.Data[0]])
		}
	}
}
