// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

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
