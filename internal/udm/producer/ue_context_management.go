package producer

import (
	"fmt"
	"net/http"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udm/producer/callback"
	"github.com/yeastengine/ella/internal/udr/producer"
)

func GetAmf3gppAccessProcedure(ueID string, supportedFeatures string) (
	response *models.Amf3GppAccessRegistration, problemDetails *models.ProblemDetails,
) {
	amf3GppAccessRegistration, err := producer.GetAmfContext3gpp(ueID)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: 404,
			Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
			Detail: err.Error(),
		}
		return nil, problemDetails
	}
	return amf3GppAccessRegistration, nil
}

// TS 29.503 5.3.2.2.2
func EditRegistrationAmf3gppAccess(registerRequest models.Amf3GppAccessRegistration, ueID string) error {
	// TODO: EPS interworking with N26 is not supported yet in this stage
	var oldAmf3GppAccessRegContext *models.Amf3GppAccessRegistration
	if context.UDM_Self().UdmAmf3gppRegContextExists(ueID) {
		ue, _ := context.UDM_Self().UdmUeFindBySupi(ueID)
		oldAmf3GppAccessRegContext = ue.Amf3GppAccessRegistration
	}

	context.UDM_Self().CreateAmf3gppRegContext(ueID, registerRequest)

	err := producer.CreateAmfContext3gpp(ueID, registerRequest)
	if err != nil {
		return fmt.Errorf("CreateAmfContext3gpp failed: %s", err)
	}

	// TS 23.502 4.2.2.2.2 14d: UDM initiate a Nudm_UECM_DeregistrationNotification to the old AMF
	// corresponding to the same (e.g. 3GPP) access, if one exists
	if oldAmf3GppAccessRegContext != nil {
		deregistData := models.DeregistrationData{
			DeregReason: models.DeregistrationReason_SUBSCRIPTION_WITHDRAWN,
			AccessType:  models.AccessType__3_GPP_ACCESS,
		}
		callback.SendOnDeregistrationNotification(ueID, oldAmf3GppAccessRegContext.DeregCallbackUri,
			deregistData) // Deregistration Notify Triggered

		return nil
	} else {
		header := make(http.Header)
		udmUe, _ := context.UDM_Self().UdmUeFindBySupi(ueID)
		header.Set("Location", udmUe.GetLocationURI(context.LocationUriAmf3GppAccessRegistration))
		return nil
	}
}
