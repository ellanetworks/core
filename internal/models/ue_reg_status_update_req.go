package models

type UeRegStatusUpdateReqData struct {
	TransferStatus       UeContextTransferStatus
	ToReleaseSessionList []int32
	PcfReselectedInd     bool
}
