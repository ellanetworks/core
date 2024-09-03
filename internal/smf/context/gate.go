package context

const (
	GateOpen uint8 = iota
	GateClose
)

type GateStatus struct {
	ULGate uint8 // 0x00001100
	DLGate uint8 // 0x00000011
}
