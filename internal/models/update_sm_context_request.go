package models

type UpdateSmContextRequest struct {
	JsonData                  *SmContextUpdateData
	BinaryDataN1SmMessage     []byte
	BinaryDataN2SmInformation []byte
}
