package util

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
)

func TACConfigToModels(intString string) (hexString string) {
	tmp, err := strconv.ParseUint(intString, 10, 32)
	if err != nil {
		logger.AmfLog.Errorf("ParseUint error: %+v", err)
		return
	}
	hexString = fmt.Sprintf("%06x", tmp)
	return
}
