package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ngap/setup_failure/unknown_plmn",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runNGSetupFailureUnknownPLMN,
	})
}

func runNGSetupFailureUnknownPLMN(_ context.Context, env scenarios.Env, _ any) error {
	g := env.FirstGNB()

	node, err := gnb.Start(&gnb.StartOpts{
		GnbID:           fmt.Sprintf("%06x", 1),
		MCC:             "002", // Unknown PLMN to trigger failure.
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    "0.0.0.0",
	})
	if err != nil {
		return fmt.Errorf("start gNB: %w", err)
	}

	defer node.Close()

	frame, err := node.WaitForMessage(
		ngapType.NGAPPDUPresentUnsuccessfulOutcome,
		ngapType.UnsuccessfulOutcomePresentNGSetupFailure,
		200*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("wait NGSetupFailure: %w", err)
	}

	if err := testutil.ValidateSCTP(frame.Info, 60, 0); err != nil {
		return fmt.Errorf("SCTP validation: %w", err)
	}

	pdu, err := ngap.Decoder(frame.Data)
	if err != nil {
		return fmt.Errorf("decode NGAP: %w", err)
	}

	if pdu.UnsuccessfulOutcome == nil {
		return fmt.Errorf("NGAP PDU is not UnsuccessfulOutcome")
	}

	if pdu.UnsuccessfulOutcome.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup {
		return fmt.Errorf("NGAP ProcedureCode is not NGSetup (%d)", ngapType.ProcedureCodeNGSetup)
	}

	failure := pdu.UnsuccessfulOutcome.Value.NGSetupFailure
	if failure == nil {
		return fmt.Errorf("NGSetupFailure is nil")
	}

	return validateNGSetupFailure(failure, ngapType.CausePresentMisc, ngapType.CauseMiscPresentUnknownPLMN)
}

func validateNGSetupFailure(failure *ngapType.NGSetupFailure, expType int, expValue aper.Enumerated) error {
	var cause *ngapType.Cause

	for _, ie := range failure.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
		default:
			return fmt.Errorf("NGSetupFailure IE ID (%d) not supported", ie.Id.Value)
		}
	}

	return validateCause(cause, expType, expValue)
}

func validateCause(cause *ngapType.Cause, expType int, expValue aper.Enumerated) error {
	if cause == nil {
		return fmt.Errorf("cause is missing")
	}

	if cause.Present != expType {
		return fmt.Errorf("cause Present: got %d, want %d", cause.Present, expType)
	}

	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		if cause.RadioNetwork.Value != expValue {
			return fmt.Errorf("radio network cause: got %d, want %d", cause.RadioNetwork.Value, expValue)
		}
	case ngapType.CausePresentTransport:
		if cause.Transport.Value != expValue {
			return fmt.Errorf("transport cause: got %d, want %d", cause.Transport.Value, expValue)
		}
	case ngapType.CausePresentNas:
		if cause.Nas.Value != expValue {
			return fmt.Errorf("nas cause: got %d, want %d", cause.Nas.Value, expValue)
		}
	case ngapType.CausePresentProtocol:
		if cause.Protocol.Value != expValue {
			return fmt.Errorf("protocol cause: got %d, want %d", cause.Protocol.Value, expValue)
		}
	case ngapType.CausePresentMisc:
		if cause.Misc.Value != expValue {
			return fmt.Errorf("misc cause: got %d, want %d", cause.Misc.Value, expValue)
		}
	default:
		return fmt.Errorf("unexpected cause Present: %d", cause.Present)
	}

	return nil
}
