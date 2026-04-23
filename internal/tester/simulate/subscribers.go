package simulate

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/core"
)

const (
	defaultPolicyName  = "default"
	defaultProfileName = "default"
	defaultSliceName   = "default"
)

// buildSubscriberConfigs generates n sequential subscriber configs starting from startIMSI.
// Each subscriber shares the same key, OPC, sequence number, and policy name.
func buildSubscriberConfigs(n int, startIMSI, key, opc, sqn string) ([]core.SubscriberConfig, error) {
	subs := make([]core.SubscriberConfig, 0, n)

	for i := range n {
		imsi, err := incrementIMSI(startIMSI, i)
		if err != nil {
			return nil, fmt.Errorf("failed to compute IMSI for index %d: %v", i, err)
		}

		subs = append(subs, core.SubscriberConfig{
			Imsi:           imsi,
			Key:            key,
			SequenceNumber: sqn,
			OPc:            opc,
			ProfileName:    defaultProfileName,
		})
	}

	return subs, nil
}

// incrementIMSI returns a zero-padded 15-digit IMSI equal to baseIMSI + increment.
func incrementIMSI(baseIMSI string, increment int) (string, error) {
	base, err := strconv.Atoi(baseIMSI)
	if err != nil {
		return "", fmt.Errorf("failed to parse base IMSI %q: %v", baseIMSI, err)
	}

	return fmt.Sprintf("%015d", base+increment), nil
}
