package context

import (
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func radioRemoteAddr(r *Radio) string {
	if r == nil || r.Conn == nil || r.Conn.RemoteAddr() == nil {
		return ""
	}

	return r.Conn.RemoteAddr().String()
}

// IdentityFields returns a canonical field set for this UE.
func (ue *AmfUe) IdentityFields() []zap.Field {
	if ue == nil {
		return nil
	}

	var (
		amfUeNgapID int64
		ranUeNgapID int64
		ranAddr     string
	)

	if ue.RanUe != nil {
		amfUeNgapID = ue.RanUe.AmfUeNgapID
		ranUeNgapID = ue.RanUe.RanUeNgapID
		ranAddr = radioRemoteAddr(ue.RanUe.Radio)
	}

	return logger.UEIdentityFields(ue.Supi.String(), ue.Guti.String(), amfUeNgapID, ranUeNgapID, ranAddr)
}

// ScopedLog returns an AMF logger enriched with canonical UE identity fields.
func (ue *AmfUe) ScopedLog() *zap.Logger {
	if ue == nil {
		return logger.AmfLog
	}

	return logger.AmfLog.With(ue.IdentityFields()...)
}

// IdentityFields returns a canonical field set for this RAN UE.
func (ranUe *RanUe) IdentityFields() []zap.Field {
	if ranUe == nil {
		return nil
	}

	supi := ""
	guti := ""

	if ranUe.AmfUe != nil {
		supi = ranUe.AmfUe.Supi.String()
		guti = ranUe.AmfUe.Guti.String()
	}

	return logger.UEIdentityFields(supi, guti, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, radioRemoteAddr(ranUe.Radio))
}

// ScopedLog returns an AMF logger enriched with canonical RAN UE identity fields.
func (ranUe *RanUe) ScopedLog() *zap.Logger {
	if ranUe == nil {
		return logger.AmfLog
	}

	return logger.AmfLog.With(ranUe.IdentityFields()...)
}

// RefreshLoggers rebuilds UE and RAN UE scoped loggers to keep identity context in sync.
func (ranUe *RanUe) RefreshLoggers() {
	if ranUe == nil {
		return
	}

	ranUe.Log = ranUe.ScopedLog()
	if ranUe.AmfUe != nil {
		ranUe.AmfUe.Log = ranUe.AmfUe.ScopedLog()
	}
}
