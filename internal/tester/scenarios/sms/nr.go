// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/smsue"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

type nr struct {
	g       *gnb.GnodeB
	nextRAN atomic.Int64
}

type nrPhone struct {
	g      *gnb.GnodeB
	ue     *ue.UE
	ranID  int64
	msisdn string
}

func startNR(env scenarios.Env) (*nr, error) {
	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester-SMS-gNB",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return nil, fmt.Errorf("start gNB: %w", err)
	}

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, scenarios.NGSetupTimeout); err != nil {
		gNodeB.Close()
		return nil, fmt.Errorf("await NG Setup Response: %w", err)
	}

	return &nr{g: gNodeB}, nil
}

func (n *nr) Name() string { return "5G" }

func (n *nr) Close() { n.g.Close() }

func (n *nr) AwaitPage(timeout time.Duration) error {
	_, err := n.g.WaitForMessage(gnb.Initiating, ngap.ProcPaging, timeout)
	return err
}

func (n *nr) Attach(imsi, msisdn string) (phone, error) {
	p, err := n.register(imsi, msisdn, true)
	if err != nil {
		return nil, err
	}

	if !p.ue.SMSAllowed() {
		return nil, fmt.Errorf("registration accepted without SMS over NAS")
	}

	if err := p.Connect(); err != nil {
		return nil, err
	}

	return p, nil
}

func (n *nr) register(imsi, msisdn string, requestSMS bool) (*nrPhone, error) {
	u, err := ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		GnodeB:         n.g,
		Msin:           imsi[len(scenarios.DefaultMCC)+len(scenarios.DefaultMNC):],
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
		NoAutoPDUSession: true,
		RequestSMS:       requestSMS,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
	})
	if err != nil {
		return nil, err
	}

	ranID := n.nextRAN.Add(1)
	n.g.AddUE(ranID, u)

	if err := n.g.RegisterWithoutSession(u, ranID, registrationTimeout); err != nil {
		return nil, err
	}

	if err := u.WaitForRRCRelease(releaseTimeout); err != nil {
		return nil, fmt.Errorf("await release after registration: %w", err)
	}

	return &nrPhone{g: n.g, ue: u, ranID: ranID, msisdn: msisdn}, nil
}

func (p *nrPhone) Stack() *smsue.Stack { return p.ue.SMS }

func (p *nrPhone) Number() string { return p.msisdn }

func (p *nrPhone) Connect() error {
	return p.g.ServiceRequestSignalling(p.ue, p.ranID, registrationTimeout)
}

func (p *nrPhone) Idle() error {
	return p.g.ReleaseContext(p.ue, p.ranID, nil, gnb.CauseUserInactivity, releaseTimeout)
}

func (p *nrPhone) AnswerPage() error {
	if err := p.ue.SendServiceRequest(p.ranID, [16]bool{}, uint8(fgs.ServiceTypeMobileTerminatedServices)); err != nil {
		return err
	}

	_, err := p.ue.WaitForNASGMMMessage(uint8(fgs.MsgServiceAccept), registrationTimeout)

	return err
}

func (p *nrPhone) Close() {}
