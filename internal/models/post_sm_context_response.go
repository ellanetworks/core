package models

type PostSmContextsResponse struct {
	JsonData                  *SmContextCreatedData `json:"jsonData,omitempty" multipart:"contentType:application/json"`
	BinaryDataN2SmInformation []byte                `json:"binaryDataN2SmInformation,omitempty" multipart:"contentType:application/vnd.3gpp.ngap,ref:JsonData.N2SmInfo.ContentId"`
}
