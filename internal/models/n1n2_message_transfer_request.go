package models

type N1N2MessageTransferRequest struct {
	JSONData                *N1N2MessageTransferReqData
	BinaryDataN1Message     []byte
	BinaryDataN2Information []byte
}
