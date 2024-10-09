package nas

import (
	"errors"
	"fmt"

	"github.com/omec-project/nas"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/context"
)

func Dispatch(ue *context.AmfUe, accessType models.AccessType, procedureCode int64, msg *nas.Message) error {
	if msg.GmmMessage == nil {
		return errors.New("gmm Message is nil")
	}

	if msg.GsmMessage != nil {
		return errors.New("GSM Message should include in GMM Message")
	}

	if ue.State[accessType] == nil {
		return fmt.Errorf("UE State is empty (accessType=%q). Can't send GSM Message", accessType)
	}

	return nil
}
