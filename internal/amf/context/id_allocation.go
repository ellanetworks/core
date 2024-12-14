package context

import (
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/util/idgenerator"
)

func AllocateUniqueID(generator **idgenerator.IDGenerator, idName string) (int64, error) {
	val, err := (*generator).Allocate()
	if err != nil {
		logger.AmfLog.Warnf("Max IDs generated for Instance")
		return -1, err
	}

	return val, nil
}
