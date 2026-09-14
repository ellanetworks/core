// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	_ "embed"

	"github.com/ellanetworks/core/s1ap"
)

//go:embed capture/source_to_target.bin
var sourceToTargetCapture []byte

//go:embed capture/target_to_source.bin
var targetToSourceCapture []byte

//go:embed capture/enb_status_transfer.bin
var enbStatusTransferCapture []byte

func SourceToTargetContainer() s1ap.TransparentContainer {
	return s1ap.TransparentContainer(sourceToTargetCapture)
}

func TargetToSourceContainer() s1ap.TransparentContainer {
	return s1ap.TransparentContainer(targetToSourceCapture)
}

func ENBStatusTransferContainer() s1ap.StatusTransferContainer {
	return s1ap.StatusTransferContainer(enbStatusTransferCapture)
}
