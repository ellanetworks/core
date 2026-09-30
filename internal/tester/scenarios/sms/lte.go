// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/smsue"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

const s1MMEPort = "36412"

type lte struct {
	enb *s1enb.ENB
}

type ltePhone struct {
	net    *lte
	enb    *s1enb.ENB
	ue     *s1enb.UE
	msisdn string
	mme    int64
	conn   int64
	guti   *eps.EPSMobileIdentity
	ebi    uint8
	stop   func()
}

func startLTE(env scenarios.Env) (*lte, error) {
	return startLTEAt(env, env.FirstCore(), 0)
}

func startLTEAt(env scenarios.Env, core string, index int) (*lte, error) {
	host, _, err := net.SplitHostPort(core)
	if err != nil {
		return nil, fmt.Errorf("parse core N2 address %q: %w", core, err)
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return nil, err
	}

	g := env.FirstGNB()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(enbID) + uint32(index), // #nosec G115 -- index is a small core position
		MCC:              scenarios.DefaultMCC,
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             fmt.Sprintf("Ella-Core-Tester-SMS-eNB-%d", index),
		CoreS1MMEAddress: net.JoinHostPort(host, s1MMEPort),
		ENBAddress:       g.N2Address,
		ENBN3Address:     g.N3Address,
	})
	if err != nil {
		return nil, fmt.Errorf("start eNB: %w", err)
	}

	return &lte{enb: e}, nil
}

func (l *lte) Name() string { return "4G" }

func (l *lte) NodeType() string { return "mme" }

func (l *lte) Close() { _ = l.enb.Close() }

func (l *lte) AwaitPage(timeout time.Duration) error {
	_, err := l.enb.WaitForPaging(timeout)
	return err
}

func (l *lte) Attach(imsi, msisdn string) (phone, error) {
	res, ue, err := l.attach(imsi, true)
	if err != nil {
		return nil, err
	}

	if !res.SMSOnly {
		return nil, fmt.Errorf("combined attach accepted without \"SMS only\" (result %s, cause %v)", res.AttachResultValue, res.EMMCause)
	}

	p := &ltePhone{net: l, enb: l.enb, msisdn: msisdn}
	p.adopt(ue, res)

	return p, nil
}

func (l *lte) attach(imsi string, smsOnly bool) (*s1enb.AttachResult, *s1enb.UE, error) {
	k, err := hex.DecodeString(scenarios.DefaultKey)
	if err != nil {
		return nil, nil, err
	}

	opc, err := hex.DecodeString(scenarios.DefaultOPC)
	if err != nil {
		return nil, nil, err
	}

	ue := l.enb.NewUE(imsi, [16]byte(k), [16]byte(opc))
	if smsOnly {
		ue.RequestSMSOnly()
	}

	res, err := l.enb.Attach(ue, registrationTimeout)
	if err != nil {
		return nil, nil, err
	}

	return res, ue, nil
}

func (p *ltePhone) adopt(ue *s1enb.UE, res *s1enb.AttachResult) {
	p.ue, p.mme, p.conn, p.guti, p.ebi = ue, res.MMEUES1APID, res.ENBUES1APID, res.GUTI, uint8(res.ERABID)
	p.stop = p.enb.ServeNAS(ue, p.mme, p.conn)
}

func (p *ltePhone) Stack() *smsue.Stack { return p.ue.SMS }

func (p *ltePhone) Number() string { return p.msisdn }

func (p *ltePhone) Idle() error {
	p.halt()

	return p.enb.ReleaseContext(p.mme, p.conn, s1enb.CauseUserInactivity, releaseTimeout)
}

func (p *ltePhone) Connect() error {
	return p.serviceRequest(false)
}

func (p *ltePhone) AnswerPage() error {
	return p.serviceRequest(true)
}

func (p *ltePhone) serviceRequest(answeringPage bool) error {
	p.halt()

	request := p.enb.ServiceRequest
	if answeringPage {
		request = p.enb.ServiceRequestAnsweringPage
	}

	res, err := request(p.ue, p.guti, registrationTimeout, nil)
	if err != nil {
		return err
	}

	p.mme, p.conn, p.guti = res.MMEUES1APID, res.ENBUES1APID, res.GUTI
	p.stop = p.enb.ServeNAS(p.ue, p.mme, p.conn)

	return nil
}

func (p *ltePhone) halt() {
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}
}

func (p *ltePhone) UpdateRegistration() (bool, error) {
	if err := p.Idle(); err != nil {
		return false, err
	}

	var status nas.EPSBearerContextStatus

	status.Active[p.ebi] = true

	accept, err := p.enb.CombinedTrackingAreaUpdate(p.ue, p.guti, &status, registrationTimeout)
	if err != nil {
		return false, err
	}

	if accept.GUTI != nil {
		p.guti = accept.GUTI
	}

	if accept.EPSBearerContextStatus == nil || !accept.EPSBearerContextStatus.Active[p.ebi] {
		return false, fmt.Errorf("the network dropped the UE's default bearer %d (status %+v)", p.ebi, accept.EPSBearerContextStatus)
	}

	if accept.EPSUpdateResult == eps.EPSUpdateResultCombined && accept.AdditionalUpdateResult != nil {
		return true, nil
	}

	if accept.Cause == nil || *accept.Cause != eps.EMMCauseCSDomainNotAvailable {
		return false, fmt.Errorf("combined TAU answered EPS only with EMM cause %v, want #18", accept.Cause)
	}

	return false, nil
}

func (p *ltePhone) AwaitAvailable(time.Duration) error { return nil }

func (p *ltePhone) RegainSMS() error {
	if err := p.Connect(); err != nil {
		return fmt.Errorf("connect before switching off: %w", err)
	}

	if err := p.SwitchOff(); err != nil {
		return err
	}

	res, ue, err := p.net.attach(p.ue.IMSI, true)
	if err != nil {
		return fmt.Errorf("attach after switching on: %w", err)
	}

	if !res.SMSOnly {
		return fmt.Errorf("combined attach after switching on accepted without \"SMS only\" (result %s, cause %v)", res.AttachResultValue, res.EMMCause)
	}

	p.adopt(ue, res)

	return nil
}

func (p *ltePhone) SwitchOff() error {
	p.halt()

	return p.enb.SwitchOff(p.ue, p.mme, p.conn, releaseTimeout)
}

func (p *ltePhone) Close() { p.halt() }
