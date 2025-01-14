// Copyright 2024 Ella Networks

package context

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/idgenerator"
)

func AllocateUniqueID(generator **idgenerator.IDGenerator, idName string) (int64, error) {
	val, err := (*generator).Allocate()
	if err != nil {
		logger.AmfLog.Warnf("Max IDs generated for Instance")
		return -1, err
	}

	return val, nil
}
