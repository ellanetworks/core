package udm

import (
	"github.com/omec-project/openapi/models"
)

// TS 29.503 5.3.2.2.2
func EditRegistrationAmf3gppAccess(registerRequest models.Amf3GppAccessRegistration, ueID string) error {
	udmContext.CreateAmf3gppRegContext(ueID, registerRequest)
	return nil
}
