// Copyright 2024 Ella Networks

package util

import (
	"fmt"
	"strconv"
)

func TACConfigToModels(intString string) (string, error) {
	tmp, err := strconv.ParseUint(intString, 10, 32)
	if err != nil {
		return "", fmt.Errorf("error parsing TAC: %+v", err)
	}

	return fmt.Sprintf("%06x", tmp), nil
}
