package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
)

func plmnIDStringToModels(plmnIDStr string) models.PlmnID {
	var plmnID models.PlmnID
	plmnID.Mcc = plmnIDStr[:3]
	plmnID.Mnc = plmnIDStr[3:]
	return plmnID
}

func getAndSetSubscriberData(ctx context.Context, ue *amfContext.AmfUe) error {
	bitRate, dnn, err := amfContext.GetSubscriberData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data: %v", err)
	}

	ue.Dnn = dnn
	ue.Ambr = bitRate

	return nil
}
