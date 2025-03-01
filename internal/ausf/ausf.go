// Copyright 2024 Ella Networks

package ausf

import (
	"fmt"
	"regexp"
)

func Start() error {
	snRegex, err := regexp.Compile("5G:mnc[0-9]{3}[.]mcc[0-9]{3}[.]3gppnetwork[.]org")
	if err != nil {
		return fmt.Errorf("could not compile SN regex: %v", err)
	}
	ausfContext.snRegex = snRegex

	return nil
}
