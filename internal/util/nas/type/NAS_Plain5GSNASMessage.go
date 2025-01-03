// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType

// Plain5GSNASMessage 9.9
type Plain5GSNASMessage struct{}

func NewPlain5GSNASMessage() (plain5GSNASMessage *Plain5GSNASMessage) {
	plain5GSNASMessage = &Plain5GSNASMessage{}
	return plain5GSNASMessage
}
