// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

func decodeUserLocationInformation(in *ngapType.UserLocationInformation) (UserLocationInformation, bool) {
	if in == nil {
		return UserLocationInformation{}, false
	}

	switch in.Present {
	case ngapType.UserLocationInformationPresentUserLocationInformationNR:
		if in.UserLocationInformationNR == nil {
			return UserLocationInformation{}, false
		}

		return UserLocationInformation{kind: UserLocationKindNR, raw: in}, true

	case ngapType.UserLocationInformationPresentUserLocationInformationEUTRA:
		if in.UserLocationInformationEUTRA == nil {
			return UserLocationInformation{}, false
		}

		return UserLocationInformation{kind: UserLocationKindEUTRA, raw: in}, true

	case ngapType.UserLocationInformationPresentUserLocationInformationN3IWF:
		if in.UserLocationInformationN3IWF == nil {
			return UserLocationInformation{}, false
		}

		return UserLocationInformation{kind: UserLocationKindN3IWF, raw: in}, true

	default:
		return UserLocationInformation{}, false
	}
}
