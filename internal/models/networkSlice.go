package models

type GNodeB struct {
	Name string
	Tac  int32
}

type UPF struct {
	Name string
	Port int
}

type NetworkSlice struct {
	Mcc     string
	Mnc     string
	GNodeBs []GNodeB
	Upf     UPF
}
