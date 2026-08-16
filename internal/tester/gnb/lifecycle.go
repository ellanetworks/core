// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

// PDUSessionResult reports one PDU session the network established: the
// addresses the UE was assigned and the N3 endpoint the gNB reported to the
// AMF. s1enb.PDNResult is its EPS counterpart.
//
// It is a snapshot. The gNB reallocates a downlink TEID every time the network
// re-establishes the session (service request, handover), so a scenario tears
// down the tunnel it built with the DLTEID it built it from, never with a value
// read back later.
type PDUSessionResult struct {
	PDUSessionID uint8
	UEIPv4       string
	UEIPv6       string
	MTU          uint16
	// QFI identifies the QoS flow in the GTP-U PDU Session Container
	// (TS 38.415). EPS has no per-flow identifier, so s1enb.PDNResult has none.
	QFI uint8
	// Accept is the plaintext PDU SESSION ESTABLISHMENT ACCEPT that opened it.
	Accept              []byte
	FiveQI              int64 // QCI on EPS
	ARP                 int64
	SessAmbrUplinkBps   int64
	SessAmbrDownlinkBps int64
	UpfAddress          string // UPF N3 address (uplink target)
	ULTEID              uint32 // UPF uplink TEID
	DLTEID              uint32 // gNB downlink TEID reported to the AMF
}

// RegistrationResult reports the NGAP identifiers and the PDU session an
// initial registration established. s1enb.AttachResult is its EPS counterpart.
type RegistrationResult struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	Session     PDUSessionResult
}

// ServiceRequestResult reports the NGAP identifiers and the re-established N3
// endpoint from a completed service request. s1enb.ServiceRequestResult is its
// EPS counterpart.
type ServiceRequestResult struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	Session     PDUSessionResult
}

// Register drives a full initial registration with the UE's default PDU session
// (TS 23.502 §4.2.2.2), returning once the network has sent its Configuration
// Update Command. ENB.Attach is its EPS counterpart.
func (g *GnodeB) Register(u *ue.UE, ranUENGAPID int64, pduSessionID uint8, timeout time.Duration) (*RegistrationResult, error) {
	generation := g.sessionGeneration()

	if err := u.SendRegistrationRequest(ranUENGAPID, uint8(fgs.RegistrationTypeInitial)); err != nil {
		return nil, fmt.Errorf("send Registration Request: %w", err)
	}

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), timeout); err != nil {
		return nil, fmt.Errorf("await Registration Accept: %w", err)
	}

	accept, err := u.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentAccept), timeout)
	if err != nil {
		return nil, fmt.Errorf("await PDU Session Establishment Accept: %w", err)
	}

	session, err := g.awaitSession(u, ranUENGAPID, pduSessionID, generation, timeout)
	if err != nil {
		return nil, err
	}

	session.Accept = accept

	// The store happens before the gNB answers, so let the PDU Session Resource
	// Setup Response reach the AMF before the caller drives the next procedure.
	time.Sleep(setupResponseSettle)

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgConfigurationUpdateCommand), timeout); err != nil {
		return nil, fmt.Errorf("await Configuration Update Command: %w", err)
	}

	return &RegistrationResult{
		AMFUENGAPID: g.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
		Session:     session,
	}, nil
}

// EstablishPDUSession opens an additional PDU session on a registered UE
// (TS 23.502 §4.3.2.2.1). ENB.OpenPDN is its EPS counterpart.
func (g *GnodeB) EstablishPDUSession(u *ue.UE, ranUENGAPID int64, pduSessionID uint8, dnn string, snssai models.Snssai, timeout time.Duration) (*PDUSessionResult, error) {
	return g.openPDUSession(u, ranUENGAPID, pduSessionID, dnn, snssai, timeout, u.SendPDUSessionEstablishmentRequest)
}

// MovePDUSessionFromEPS requests an existing EPS PDN connection over NR as a PDU
// session (TS 23.502 §4.11.2.2), the idle-mode inter-system change without N26.
// It differs from EstablishPDUSession only in the NAS Request Type.
func (g *GnodeB) MovePDUSessionFromEPS(u *ue.UE, ranUENGAPID int64, pduSessionID uint8, dnn string, snssai models.Snssai, timeout time.Duration) (*PDUSessionResult, error) {
	return g.openPDUSession(u, ranUENGAPID, pduSessionID, dnn, snssai, timeout, u.MovePDUSessionFromEPC)
}

type pduSessionRequest func(amfUENGAPID, ranUENGAPID int64, pduSessionID uint8, dnn string, snssai models.Snssai) error

func (g *GnodeB) openPDUSession(u *ue.UE, ranUENGAPID int64, pduSessionID uint8, dnn string, snssai models.Snssai, timeout time.Duration, request pduSessionRequest) (*PDUSessionResult, error) {
	generation := g.sessionGeneration()

	if err := request(g.GetAMFUENGAPID(ranUENGAPID), ranUENGAPID, pduSessionID, dnn, snssai); err != nil {
		return nil, fmt.Errorf("send the request for PDU session %d: %w", pduSessionID, err)
	}

	accept, err := u.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentAccept), timeout)
	if err != nil {
		return nil, fmt.Errorf("await PDU Session Establishment Accept for session %d: %w", pduSessionID, err)
	}

	session, err := g.awaitSession(u, ranUENGAPID, pduSessionID, generation, timeout)
	if err != nil {
		return nil, err
	}

	session.Accept = accept

	time.Sleep(setupResponseSettle)

	return &session, nil
}

// MobilityRegistrationUpdate re-registers a UE the AMF already knows under a new
// RAN UE NGAP ID (TS 23.502 §4.2.2.2.2), re-establishing its user plane.
// ENB.PeriodicTrackingAreaUpdate is the closest EPS counterpart.
func (g *GnodeB) MobilityRegistrationUpdate(u *ue.UE, ranUENGAPID int64, pduSessionID uint8, timeout time.Duration) (*RegistrationResult, error) {
	generation := g.sessionGeneration()

	g.AddUE(ranUENGAPID, u)

	if err := u.SendMobilityRegistrationRequest(ranUENGAPID, []uint8{pduSessionID}); err != nil {
		return nil, fmt.Errorf("send Registration Request (mobility updating): %w", err)
	}

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), timeout); err != nil {
		return nil, fmt.Errorf("await Registration Accept for the mobility registration update: %w", err)
	}

	session, err := g.awaitSession(u, ranUENGAPID, pduSessionID, generation, timeout)
	if err != nil {
		return nil, err
	}

	return &RegistrationResult{
		AMFUENGAPID: g.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
		Session:     session,
	}, nil
}

// ServiceRequest performs a mobile-originated service request for a UE in
// CM-IDLE (TS 23.502 §4.2.3.2), re-establishing the PDU session's user plane.
// ENB.ServiceRequest is its EPS counterpart.
func (g *GnodeB) ServiceRequest(u *ue.UE, ranUENGAPID int64, pduSessionID uint8, timeout time.Duration) (*ServiceRequestResult, error) {
	var status [16]bool

	status[pduSessionID] = true

	if err := u.SendServiceRequest(ranUENGAPID, status, uint8(fgs.ServiceTypeData)); err != nil {
		return nil, fmt.Errorf("send Service Request: %w", err)
	}

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgServiceAccept), timeout); err != nil {
		return nil, fmt.Errorf("await Service Accept: %w", err)
	}

	// Generation 0: accept whatever the gNB now holds. The AMF re-activates the
	// user plane in the Initial Context Setup Request that carries the Service
	// Accept, but it only re-sends PDU Session Resource Setup when the AN tunnel
	// actually changes; otherwise the session keeps the endpoint it already had.
	// Either way the store is current, because the handler stores before it hands
	// the Service Accept to the UE.
	session, err := g.awaitSession(u, ranUENGAPID, pduSessionID, 0, timeout)
	if err != nil {
		return nil, err
	}

	return &ServiceRequestResult{
		AMFUENGAPID: g.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
		Session:     session,
	}, nil
}

// ReleaseContext performs a gNB-initiated UE context release (TS 23.502
// §4.2.6), dropping the UE to CM-IDLE. ENB.ReleaseContext is its EPS
// counterpart.
func (g *GnodeB) ReleaseContext(u *ue.UE, ranUENGAPID int64, pduSessionIDs []uint8, timeout time.Duration) error {
	var status [16]bool

	for _, id := range pduSessionIDs {
		status[id] = true
	}

	err := g.SendUEContextReleaseRequest(&UEContextReleaseRequestOpts{
		AMFUENGAPID:   g.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		PDUSessionIDs: status,
		Cause:         ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkReleaseDueToNGRANGeneratedReason},
	})
	if err != nil {
		return fmt.Errorf("send UE Context Release Request: %w", err)
	}

	if err := u.WaitForRRCRelease(timeout); err != nil {
		return fmt.Errorf("await RRC Release for %s: %w", u.UeSecurity.Supi, err)
	}

	return nil
}

// Deregister performs a UE-originating deregistration (TS 23.502 §4.2.2.3.2).
// ENB.Detach is its EPS counterpart.
func (g *GnodeB) Deregister(u *ue.UE, ranUENGAPID int64, timeout time.Duration) error {
	if err := u.SendDeregistrationRequest(g.GetAMFUENGAPID(ranUENGAPID), ranUENGAPID); err != nil {
		return fmt.Errorf("send Deregistration Request: %w", err)
	}

	if err := u.WaitForRRCRelease(timeout); err != nil {
		return fmt.Errorf("await RRC Release for %s: %w", u.UeSecurity.Supi, err)
	}

	return nil
}

// PDUSession reports the gNB's current view of a stored PDU session, by value.
// Use it to observe what a network-initiated modification changed; a scenario
// that wants the session it just established reads it off the procedure result
// instead, which is pinned to that establishment and cannot drift.
func (g *GnodeB) PDUSession(ranUENGAPID int64, pduSessionID uint8) (PDUSessionResult, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	s, ok := g.pduSessions[ranUENGAPID][int64(pduSessionID)]
	if !ok {
		return PDUSessionResult{}, false
	}

	return PDUSessionResult{
		PDUSessionID:        pduSessionID,
		QFI:                 uint8(s.QFI),
		FiveQI:              s.FiveQi,
		ARP:                 s.PriArp,
		SessAmbrUplinkBps:   s.AmbrUplink,
		SessAmbrDownlinkBps: s.AmbrDownlink,
		UpfAddress:          s.UpfAddress,
		ULTEID:              s.ULTEID,
		DLTEID:              s.DLTEID,
	}, true
}

// setupResponseSettle lets the gNB's PDU Session Resource Setup Response reach
// the AMF: the session is stored before the response goes out, so awaiting the
// store alone would race the next procedure.
const setupResponseSettle = 50 * time.Millisecond

// awaitSession joins what the UE was told over NAS with what the gNB was told
// over NGAP into the single result a scenario needs to build its tunnel. Only
// resources established after `generation` count, so a re-established session is
// never confused with the one it replaced; pass 0 to accept the session the gNB
// currently holds.
func (g *GnodeB) awaitSession(u *ue.UE, ranUENGAPID int64, pduSessionID uint8, generation uint64, timeout time.Duration) (PDUSessionResult, error) {
	ueSession, err := u.WaitForPDUSession(pduSessionID, timeout)
	if err != nil {
		return PDUSessionResult{}, fmt.Errorf("the UE was assigned no PDU session %d: %w", pduSessionID, err)
	}

	ranSession, err := g.awaitPDUSession(ranUENGAPID, int64(pduSessionID), generation, timeout)
	if err != nil {
		return PDUSessionResult{}, fmt.Errorf("the gNB set up no resources for PDU session %d: %w", pduSessionID, err)
	}

	return PDUSessionResult{
		PDUSessionID:        pduSessionID,
		UEIPv4:              ueSession.UEIPv4,
		UEIPv6:              ueSession.UEIPv6,
		MTU:                 ueSession.MTU,
		QFI:                 ueSession.QFI,
		FiveQI:              ranSession.FiveQi,
		ARP:                 ranSession.PriArp,
		SessAmbrUplinkBps:   ranSession.AmbrUplink,
		SessAmbrDownlinkBps: ranSession.AmbrDownlink,
		UpfAddress:          ranSession.UpfAddress,
		ULTEID:              ranSession.ULTEID,
		DLTEID:              ranSession.DLTEID,
	}, nil
}
