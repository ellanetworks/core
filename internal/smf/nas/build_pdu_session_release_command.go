package nas

import (
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func BuildGSMPDUSessionReleaseCommand(pduSessionID uint8, pti uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseCommand = nasMessage.NewPDUSessionReleaseCommand(0x0)
	m.PDUSessionReleaseCommand.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	m.PDUSessionReleaseCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseCommand.SetPDUSessionID(pduSessionID)
	m.PDUSessionReleaseCommand.SetPTI(pti)
	m.PDUSessionReleaseCommand.SetCauseValue(0x0)

	return m.PlainNasEncode()
}
