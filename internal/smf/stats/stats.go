package stats

import "github.com/yeastengine/ella/internal/smf/context"

func GetPDUSessionCount() int {
	return context.GetPDUSessionCount()
}
