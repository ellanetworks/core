package models

type N1N2MessageTransferRequest struct {
	JsonData                *N1N2MessageTransferReqData `json:"jsonData,omitempty" multipart:"contentType:application/json"`
	BinaryDataN1Message     []byte                      `json:"binaryDataN1Message,omitempty" multipart:"contentType:application/vnd.3gpp.5gnas,ref:JsonData.N1MessageContainer.N1MessageContent.ContentId"`
	BinaryDataN2Information []byte                      `json:"binaryDataN2Information,omitempty" multipart:"contentType:application/vnd.3gpp.ngap,class:JsonData.N2InfoContainer.N2InformationClass,ref:(N2InfoContent).NgapData.ContentId"`
}
