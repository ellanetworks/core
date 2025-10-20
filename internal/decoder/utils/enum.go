package utils

type EnumField[T ~int | ~uint8 | ~uint16 | ~uint64 | ~int64] struct {
	Type    string `json:"type"` // always "enum"
	Value   T      `json:"value"`
	Label   string `json:"label"`
	Unknown bool   `json:"unknown"`
}

func MakeEnum[T ~int | ~uint8 | ~uint16 | ~uint64 | ~int64](v T, label string, unknown bool) EnumField[T] {
	return EnumField[T]{Type: "enum", Value: v, Label: label, Unknown: unknown}
}
