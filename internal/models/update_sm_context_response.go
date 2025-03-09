package models

type UpdateSmContextResponse struct {
	JSONData                  *SmContextUpdatedData
	BinaryDataN1SmMessage     []byte
	BinaryDataN2SmInformation []byte
}
