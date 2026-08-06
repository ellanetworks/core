// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package interworking holds the scenarios that move a subscriber's session
// between EPS and 5GS without an N26 interface (TS 23.501 §5.17.2.3).
package interworking

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

const (
	interworkingIMSI = "001010000000040"

	// transferPDUSessionID correlates the PDN connection with the PDU session
	// (TS 23.501 §5.17.2.1).
	transferPDUSessionID uint8 = 1

	ranUENGAPID int64 = 1
)

func fixture(scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers:         []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(interworkingIMSI, "")},
		AssertUsageForIMSIs: []string{interworkingIMSI},
	}
}

func startGNB(env scenarios.Env) (*gnb.GnodeB, error) {
	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester-Interworking-gNB",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return nil, fmt.Errorf("start gNB: %w", err)
	}

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, 2*time.Second); err != nil {
		gNodeB.Close()

		return nil, fmt.Errorf("await NG Setup Response: %w", err)
	}

	return gNodeB, nil
}

func startENB(env scenarios.Env) (*s1enb.ENB, error) {
	host, _, err := net.SplitHostPort(env.FirstCore())
	if err != nil {
		return nil, fmt.Errorf("parse core N2 address %q: %w", env.FirstCore(), err)
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	g := env.FirstGNB()

	return s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(enbID),
		MCC:              scenarios.DefaultMCC,
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             "Ella-Core-Tester-Interworking-eNB",
		CoreS1MMEAddress: net.JoinHostPort(host, s1mmePort),
		ENBAddress:       g.N2Address,
		ENBN3Address:     g.N3Address,
		EnableDatapath:   true,
	})
}

// s1mmePort is the standard S1-MME SCTP port (TS 36.412).
const s1mmePort = "36412"

func newFiveGUE(gNodeB *gnb.GnodeB, requestType fgs.RequestType) (*ue.UE, error) {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   transferPDUSessionID,
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		Msin:           interworkingIMSI[5:],
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.DefaultDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("create 5GS UE: %w", err)
	}

	newUE.SessionRequestType = requestType
	gNodeB.AddUE(ranUENGAPID, newUE)

	return newUE, nil
}

func newEPSUE(e *s1enb.ENB) (*s1enb.UE, error) {
	var k, opc [16]byte

	kb, err := hex.DecodeString(scenarios.DefaultKey)
	if err != nil || len(kb) != 16 {
		return nil, fmt.Errorf("invalid default key")
	}

	ob, err := hex.DecodeString(scenarios.DefaultOPC)
	if err != nil || len(ob) != 16 {
		return nil, fmt.Errorf("invalid default OPc")
	}

	copy(k[:], kb)
	copy(opc[:], ob)

	return e.NewUE(interworkingIMSI, k, opc), nil
}
