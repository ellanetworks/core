package models

type PostSmContextsRequest struct {
	JsonData              *SmContextCreateData `json:"jsonData,omitempty" multipart:"contentType:application/json"`
	BinaryDataN1SmMessage []byte               `json:"binaryDataN1SmMessage,omitempty" multipart:"contentType:application/vnd.3gpp.5gnas,ref:JsonData.N1SmMsg.ContentId"`
}
