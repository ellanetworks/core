package models

type KeyAmf struct {
	KeyType KeyAmfType `json:"keyType"`
	KeyVal  string     `json:"keyVal"`
}
