package models

type TransferReason string

// List of TransferReason
const (
	TransferReason_INIT_REG              TransferReason = "INIT_REG"
	TransferReason_MOBI_REG              TransferReason = "MOBI_REG"
	TransferReason_MOBI_REG_UE_VALIDATED TransferReason = "MOBI_REG_UE_VALIDATED"
)
