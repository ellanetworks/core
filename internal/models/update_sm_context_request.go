package models

type UpdateSmContextRequest struct {
	JSONData                  *SmContextUpdateData
	BinaryDataN1SmMessage     []byte
	BinaryDataN2SmInformation []byte
}
