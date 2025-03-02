package models

type UpdateSmContextResponse struct {
	JsonData                  *SmContextUpdatedData
	BinaryDataN1SmMessage     []byte
	BinaryDataN2SmInformation []byte
}
