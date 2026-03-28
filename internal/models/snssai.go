package models

type Snssai struct {
	Sst int32
	Sd  string
}

func (s Snssai) Equal(other Snssai) bool {
	return s.Sst == other.Sst && s.Sd == other.Sd
}
