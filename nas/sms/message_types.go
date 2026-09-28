// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//nolint:revive
package sms

func (m *CPData) MessageType() CPMessageType      { return CPMessageTypeData }
func (m *CPAck) MessageType() CPMessageType       { return CPMessageTypeAck }
func (m *CPError) MessageType() CPMessageType     { return CPMessageTypeError }
func (m *CPData) TI() TransactionIdentifier       { return m.TransactionIdentifier }
func (m *CPAck) TI() TransactionIdentifier        { return m.TransactionIdentifier }
func (m *CPError) TI() TransactionIdentifier      { return m.TransactionIdentifier }
func (m *CPData) isCPMessage()                    {}
func (m *CPAck) isCPMessage()                     {}
func (m *CPError) isCPMessage()                   {}
func (m *CPData) MarshalBinary() ([]byte, error)  { return m.AppendBinary(nil) }
func (m *CPAck) MarshalBinary() ([]byte, error)   { return m.AppendBinary(nil) }
func (m *CPError) MarshalBinary() ([]byte, error) { return m.AppendBinary(nil) }
func (m *RPData) MTI() MessageTypeIndicator       { return mti(MTIDataMSToNetwork, m.Direction) }
func (m *RPAck) MTI() MessageTypeIndicator        { return mti(MTIAckMSToNetwork, m.Direction) }
func (m *RPError) MTI() MessageTypeIndicator      { return mti(MTIErrorMSToNetwork, m.Direction) }
func (m *RPSMMA) MTI() MessageTypeIndicator       { return MTISMMAMSToNetwork }
func (m *RPData) MessageReference() uint8         { return m.Reference }
func (m *RPAck) MessageReference() uint8          { return m.Reference }
func (m *RPError) MessageReference() uint8        { return m.Reference }
func (m *RPSMMA) MessageReference() uint8         { return m.Reference }
func (m *RPData) isRPMessage()                    {}
func (m *RPAck) isRPMessage()                     {}
func (m *RPError) isRPMessage()                   {}
func (m *RPSMMA) isRPMessage()                    {}
func (m *RPData) MarshalBinary() ([]byte, error)  { return m.AppendBinary(nil) }
func (m *RPAck) MarshalBinary() ([]byte, error)   { return m.AppendBinary(nil) }
func (m *RPError) MarshalBinary() ([]byte, error) { return m.AppendBinary(nil) }
func (m *RPSMMA) MarshalBinary() ([]byte, error)  { return m.AppendBinary(nil) }

var (
	_ CPMessage = (*CPData)(nil)
	_ CPMessage = (*CPAck)(nil)
	_ CPMessage = (*CPError)(nil)
	_ RPMessage = (*RPData)(nil)
	_ RPMessage = (*RPAck)(nil)
	_ RPMessage = (*RPError)(nil)
	_ RPMessage = (*RPSMMA)(nil)
)
