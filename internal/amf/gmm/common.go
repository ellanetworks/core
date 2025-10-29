package gmm

import "github.com/ellanetworks/core/internal/amf/context"

func AttachRanUeToAmfUeAndReleaseOldIfAny(amfUe *context.AmfUe, ranUe *context.RanUe) {
	if oldRanUe := amfUe.RanUe[ranUe.Ran.AnType]; oldRanUe != nil {
		oldRanUe.DetachAmfUe()
		if amfUe.T3550 != nil {
			amfUe.State[ranUe.Ran.AnType].Set(context.Registered)
		}
	}

	amfUe.AttachRanUe(ranUe)
}
