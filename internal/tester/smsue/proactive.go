// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsue

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/tester/cat"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
)

const proactiveSubmitTimeout = 45 * time.Second

func (s *Stack) runProactive(uicc UICC) {
	s.proactive.Lock()
	defer s.proactive.Unlock()

	for {
		command, sw := uicc.Fetch()
		if sw != cat.SWOK {
			return
		}

		if !cat.ProactivePending(uicc.TerminalResponse(s.proactiveCommand(command))) {
			return
		}
	}
}

func (s *Stack) proactiveCommand(command []byte) []byte {
	tag, value, _, err := cat.ReadTLV(command)
	if err != nil || tag != cat.TagProactiveCommand {
		return terminalResponse(cat.CommandDetails{}, cat.ResultDataNotUnderstood)
	}

	objects, err := cat.ReadObjects(value)
	if err != nil {
		return terminalResponse(cat.CommandDetails{}, cat.ResultDataNotUnderstood)
	}

	raw, ok := objects.Get(cat.TagCommandDetails)
	if !ok {
		return terminalResponse(cat.CommandDetails{}, cat.ResultRequiredValuesMissing)
	}

	details, err := cat.ParseCommandDetails(raw)
	if err != nil {
		return terminalResponse(cat.CommandDetails{}, cat.ResultDataNotUnderstood)
	}

	if details.Type != cat.TypeSendShortMessage || details.Qualifier&cat.QualifierSMSPacking != 0 {
		return terminalResponse(details, cat.ResultBeyondCapabilities)
	}

	return terminalResponse(details, s.sendShortMessage(objects)...)
}

func (s *Stack) sendShortMessage(objects cat.Objects) []byte {
	submit, ok := objects.Get(cat.TagSMSTPDU)
	if !ok {
		return []byte{cat.ResultRequiredValuesMissing}
	}

	if len(submit) < 2 || submit[0]&0x03 != 0x01 && submit[0]&0x03 != 0x02 {
		return []byte{cat.ResultDataNotUnderstood}
	}

	address, ok := objects.Get(cat.TagAddress)
	if !ok {
		return []byte{cat.ResultTerminalUnable, cat.NoSpecificCause}
	}

	serviceCentre, err := sms.ParseAddress(address)
	if err != nil {
		return []byte{cat.ResultDataNotUnderstood}
	}

	s.mu.Lock()
	ref := s.nextRef
	s.nextRef++
	s.mu.Unlock()

	submit = append([]byte{}, submit...)
	submit[1] = ref

	ctx, cancel := context.WithTimeout(context.Background(), proactiveSubmitTimeout)
	defer cancel()

	report, err := s.originate(ctx, &sms.RPData{Direction: nas.DirectionUplink, Reference: ref, Destination: &serviceCentre, UserData: submit})
	if err != nil {
		return []byte{cat.ResultNetworkUnable, cat.NoSpecificCause}
	}

	if rpErr, ok := report.(*sms.RPError); ok {
		return []byte{cat.ResultSMSRPError, byte(rpErr.Cause) & 0x7f}
	}

	return []byte{cat.ResultOK}
}

func terminalResponse(details cat.CommandDetails, result ...byte) []byte {
	b := cat.AppendTLV(nil, cat.TagCommandDetails|cat.ComprehensionRequired, details.Value())
	b = cat.AppendTLV(b, cat.TagDeviceIdentities|cat.ComprehensionRequired, []byte{cat.DeviceTerminal, cat.DeviceUICC})

	return cat.AppendTLV(b, cat.TagResult|cat.ComprehensionRequired, result)
}
