package models

type UpdateSmContextErrorResponse struct {
	JsonData                  *SmContextUpdateError
	BinaryDataN1SmMessage     []byte
	BinaryDataN2SmInformation []byte
}
