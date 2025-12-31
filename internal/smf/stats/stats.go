// Copyright 2024 Ella Networks

package stats

import "github.com/ellanetworks/core/internal/smf/context"

func GetPDUSessionCount() int {
	smf := context.SMFSelf()
	return smf.GetPDUSessionCount()
}
