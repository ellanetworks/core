package context

import (
	"github.com/omec-project/util/idgenerator"
	"github.com/yeastengine/ella/internal/amf/logger"
)

func AllocateUniqueID(generator **idgenerator.IDGenerator, idName string) (int64, error) {
	val, err := (*generator).Allocate()
	if err != nil {
		logger.DataRepoLog.Warnf("Max IDs generated for Instance")
		return -1, err
	}

	return val, nil
}
