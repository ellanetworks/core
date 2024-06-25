package core

import (
	"testing"

	"github.com/wmnsk/go-pfcp/ie"
)

func TestGetTransportLevelMarking(t *testing.T) {
	// Create CreateFAR_IE with TransportLevelMarking
	CreateFAR := ie.NewCreateFAR(
		ie.NewFARID(10),
		ie.NewTransportLevelMarking(55),
	)

	tlm, err := GetTransportLevelMarking(CreateFAR)
	if err != nil {
		t.Errorf("Error getting TransportLevelMarking: %s", err.Error())
	}
	if tlm != 55 {
		t.Errorf("Expected TransportLevelMarking to be 55, got %d", tlm)
	}
}
