package models

type UpdateSmContextResponse struct {
	N2SmInfoTypePduResRel     bool
	BinaryDataN1SmMessage     []byte
	BinaryDataN2SmInformation []byte
}
