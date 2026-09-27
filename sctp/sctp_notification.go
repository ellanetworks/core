// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux && !386

// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sctp

// This file implement SCTP Notification structure defined in RFC 6458

type Notification interface {
	Type() SCTPNotificationType
	Flags() uint16
	Length() uint32
}

// SCTPAssocChangeEvent is an implementation of Notification interface
type SCTPAssocChangeEvent struct {
	sacType            uint16
	sacFlags           uint16
	sacLength          uint32
	sacState           SCTPState
	sacError           uint16
	sacOutboundStreams uint16
	sacInboundStreams  uint16
	sacAssocID         SCTPAssocID
	sacInfo            []uint8
}

func (s *SCTPAssocChangeEvent) Type() SCTPNotificationType {
	return SCTPNotificationType(s.sacType)
}

func (s *SCTPAssocChangeEvent) Flags() uint16 {
	return s.sacFlags
}

func (s *SCTPAssocChangeEvent) Length() uint32 {
	return s.sacLength
}

func (s *SCTPAssocChangeEvent) State() SCTPState {
	return s.sacState
}

func (s *SCTPAssocChangeEvent) OutboundStreams() uint16 {
	return s.sacOutboundStreams
}

func (s *SCTPAssocChangeEvent) InboundStreams() uint16 {
	return s.sacInboundStreams
}

func (s *SCTPAssocChangeEvent) AssocID() SCTPAssocID {
	return s.sacAssocID
}

func (s *SCTPAssocChangeEvent) Error() uint16 {
	return s.sacError
}

func (s *SCTPAssocChangeEvent) Info() []uint8 {
	return s.sacInfo
}

// SCTPShutdownEvent is an implementation of Notification interface
type SCTPShutdownEventNotification struct {
	sseType    uint16
	sseFlags   uint16
	sseLength  uint32
	sseAssocID SCTPAssocID
}

func (s *SCTPShutdownEventNotification) Type() SCTPNotificationType {
	return SCTPNotificationType(s.sseType)
}

func (s *SCTPShutdownEventNotification) Flags() uint16 {
	return s.sseFlags
}

func (s *SCTPShutdownEventNotification) Length() uint32 {
	return s.sseLength
}

func (s *SCTPShutdownEventNotification) AssocID() SCTPAssocID {
	return s.sseAssocID
}

// SCTPPartialDeliveryEventNotification reports that the kernel abandoned a
// partially delivered message, so its already-read prefix must be discarded.
type SCTPPartialDeliveryEventNotification struct {
	pdapiType       uint16
	pdapiFlags      uint16
	pdapiLength     uint32
	pdapiIndication uint32
}

func (p *SCTPPartialDeliveryEventNotification) Type() SCTPNotificationType {
	return SCTPNotificationType(p.pdapiType)
}

func (p *SCTPPartialDeliveryEventNotification) Flags() uint16 {
	return p.pdapiFlags
}

func (p *SCTPPartialDeliveryEventNotification) Length() uint32 {
	return p.pdapiLength
}

// Aborted reports SCTP_PARTIAL_DELIVERY_ABORTED, the only indication the kernel
// defines (include/uapi/linux/sctp.h).
func (p *SCTPPartialDeliveryEventNotification) Aborted() bool {
	return p.pdapiIndication == sctpPartialDeliveryAborted
}
