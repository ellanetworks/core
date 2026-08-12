// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import "fmt"

const (
	imsiTagValueBits = 50
	imsiTagValueMask = uint64(1)<<imsiTagValueBits - 1
	imsiTagMaxDigits = 15
)

func EncodeIMSITag(imsi string) (uint64, error) {
	if len(imsi) == 0 || len(imsi) > imsiTagMaxDigits {
		return 0, fmt.Errorf("imsi %q: length %d out of range 1..%d", imsi, len(imsi), imsiTagMaxDigits)
	}

	var value uint64

	for i := 0; i < len(imsi); i++ {
		if imsi[i] < '0' || imsi[i] > '9' {
			return 0, fmt.Errorf("imsi %q: contains non-digit characters", imsi)
		}

		value = value*10 + uint64(imsi[i]-'0')
	}

	return uint64(len(imsi))<<imsiTagValueBits | value, nil
}

func DecodeIMSITag(tag uint64) string {
	digits := int(tag >> imsiTagValueBits)
	if digits <= 0 || digits > imsiTagMaxDigits {
		return ""
	}

	return fmt.Sprintf("%0*d", digits, tag&imsiTagValueMask)
}
