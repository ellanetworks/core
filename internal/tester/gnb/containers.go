// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import _ "embed"

//go:embed synthetic/source_to_target.bin
var sourceToTargetSynthetic []byte

//go:embed synthetic/target_to_source.bin
var targetToSourceSynthetic []byte

//go:embed synthetic/ran_status_transfer.bin
var ranStatusTransferSynthetic []byte

func SourceToTargetContainer() []byte {
	return sourceToTargetSynthetic
}

func TargetToSourceContainer() []byte {
	return targetToSourceSynthetic
}

func RANStatusTransferContainer() []byte {
	return ranStatusTransferSynthetic
}
