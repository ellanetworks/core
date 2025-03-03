package models

type NasSecurityMode struct {
	IntegrityAlgorithm IntegrityAlgorithm `json:"integrityAlgorithm"`
	CipheringAlgorithm CipheringAlgorithm `json:"cipheringAlgorithm"`
}
