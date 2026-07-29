// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// parseDownlink decodes a plain downlink NAS message, so a caller can dispatch on
// its concrete type. A message type the library does not model is not a failure.
func parseDownlink(b []byte) (eps.Message, error) {
	msg, err := eps.ParseMessage(b, nas.DirectionDownlink)
	if err != nil && !nas.SoftOnly(err) {
		return nil, fmt.Errorf("parse downlink NAS: %w", err)
	}

	return msg, nil
}

// expectDownlink decodes a plain downlink NAS message and reports it as the type
// the caller expects, so a scenario fails naming the message it received.
func expectDownlink[T eps.Message](b []byte) (T, error) {
	var want T

	msg, err := parseDownlink(b)
	if err != nil {
		return want, err
	}

	got, ok := msg.(T)
	if !ok {
		return want, fmt.Errorf("expected %T, got %T", want, msg)
	}

	return got, nil
}
