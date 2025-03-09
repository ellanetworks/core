package models

type TransferReason string

const (
	TransferReasonInitReg TransferReason = "INIT_REG"
	TransferReasonMobiReg TransferReason = "MOBI_REG"
)
