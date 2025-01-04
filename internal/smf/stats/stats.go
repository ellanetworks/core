// Copyright 2024 Ella Networks

package stats

import "github.com/ellanetworks/core/internal/smf/context"

func GetPDUSessionCount() int {
	return context.GetPDUSessionCount()
}
