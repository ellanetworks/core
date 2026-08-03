// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// ParseNAS decodes a plain 5GS NAS message, so a caller can dispatch on its
// concrete type. A message type the library does not model is not a failure.
func ParseNAS(b []byte) (fgs.Message, error) {
	msg, err := fgs.ParseMessage(b)
	if err != nil && !nas.SoftOnly(err) {
		return nil, fmt.Errorf("parse NAS: %w", err)
	}

	return msg, nil
}

// ExpectNAS decodes a plain 5GS NAS message and reports it as the type the caller
// expects, so a scenario fails naming the message it received.
func ExpectNAS[T fgs.Message](b []byte) (T, error) {
	var want T

	msg, err := ParseNAS(b)
	if err != nil {
		return want, err
	}

	got, ok := msg.(T)
	if !ok {
		return want, fmt.Errorf("expected %T, got %T", want, msg)
	}

	return got, nil
}

func SDFromNAS(sd [3]uint8) string {
	return fmt.Sprintf("%x%x%x", sd[0], sd[1], sd[2])
}
