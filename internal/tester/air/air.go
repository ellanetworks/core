package air

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
)

type DownlinkSender interface {
	SendDownlinkNAS(nasPDU []byte, amfUENGAPID int64, ranUENGAPID int64) error
	RRCRelease()
}

type UplinkSender interface {
	SendUplinkNAS(nasPDU []byte, amfUENGAPID int64, ranUENGAPID int64) error
	SendInitialUEMessage(nasPDU []byte, ranUENGAPID int64, guti5G *nasType.GUTI5G, cause aper.Enumerated) error
}
