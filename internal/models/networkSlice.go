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
	Sst     int32
	Sd      string
	Mcc     string
	Mnc     string
	GNodeBs []GNodeB
	Upf     UPF
}
