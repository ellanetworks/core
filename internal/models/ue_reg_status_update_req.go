package models

type UeRegStatusUpdateReqData struct {
	TransferStatus       UeContextTransferStatus `json:"transferStatus"`
	ToReleaseSessionList []int32                 `json:"toReleaseSessionList,omitempty"`
	PcfReselectedInd     bool                    `json:"pcfReselectedInd,omitempty"`
}
