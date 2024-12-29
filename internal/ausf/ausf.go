package ausf

import (
	"regexp"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/google/uuid"
)

func Start() error {
	snRegex, err := regexp.Compile("5G:mnc[0-9]{3}[.]mcc[0-9]{3}[.]3gppnetwork[.]org")
	if err != nil {
		logger.AusfLog.Warnf("SN compile error: %+v", err)
	}
	ausfContext.snRegex = snRegex

	ausfContext.NfId = uuid.New().String()
	return nil
}
