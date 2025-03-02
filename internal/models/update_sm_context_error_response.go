package models

type UpdateSmContextErrorResponse struct {
	JsonData                  *SmContextUpdateError `json:"jsonData,omitempty" multipart:"contentType:application/json"`
	BinaryDataN1SmMessage     []byte                `json:"binaryDataN1SmMessage,omitempty" multipart:"contentType:application/vnd.3gpp.5gnas,ref:JsonData.N1SmMsg.ContentId"`
	BinaryDataN2SmInformation []byte                `json:"binaryDataN2SmInformation,omitempty" multipart:"contentType:application/vnd.3gpp.ngap,ref:JsonData.N2SmInfo.ContentId"`
}
