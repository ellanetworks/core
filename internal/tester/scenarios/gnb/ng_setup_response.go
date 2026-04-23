package gnb

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const numRadios = 24

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ngap/setup_response",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runNGSetupResponse,
	})
}

func runNGSetupResponse(_ context.Context, env scenarios.Env, _ any) error {
	eg := errgroup.Group{}

	for i := range numRadios {
		idx := i

		eg.Go(func() error { return ngSetupOneRadio(env, idx) })
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("NGSetup: %w", err)
	}

	return nil
}

func ngSetupOneRadio(env scenarios.Env, index int) error {
	g := env.FirstGNB()

	node, err := gnb.Start(&gnb.StartOpts{
		GnbID:         fmt.Sprintf("%06x", index+1),
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SST:           scenarios.DefaultSST,
		SD:            scenarios.DefaultSD,
		DNN:           scenarios.DefaultDNN,
		TAC:           scenarios.DefaultTAC,
		Name:          fmt.Sprintf("Ella-Core-Tester-%d", index),
		CoreN2Address: env.FirstCore(),
		GnbN2Address:  g.N2Address,
	})
	if err != nil {
		return fmt.Errorf("start gNB %d: %w", index, err)
	}

	defer node.Close()

	frame, err := node.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		500*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("wait NGSetupResponse: %w", err)
	}

	if err := testutil.ValidateSCTP(frame.Info, 60, 0); err != nil {
		return fmt.Errorf("SCTP validation: %w", err)
	}

	pdu, err := ngap.Decoder(frame.Data)
	if err != nil {
		return fmt.Errorf("decode NGAP: %w", err)
	}

	if pdu.SuccessfulOutcome == nil {
		return fmt.Errorf("NGAP PDU is not SuccessfulOutcome")
	}

	if pdu.SuccessfulOutcome.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup {
		return fmt.Errorf("NGAP ProcedureCode is not NGSetup (%d)", ngapType.ProcedureCodeNGSetup)
	}

	resp := pdu.SuccessfulOutcome.Value.NGSetupResponse
	if resp == nil {
		return fmt.Errorf("NGSetupResponse is nil")
	}

	return validateNGSetupResponse(resp, scenarios.DefaultMCC, scenarios.DefaultMNC, scenarios.DefaultSST, scenarios.DefaultSD)
}

func validateNGSetupResponse(resp *ngapType.NGSetupResponse, expMCC, expMNC string, expSST int, expSD string) error {
	var (
		amfName             *ngapType.AMFName
		guamiList           *ngapType.ServedGUAMIList
		relativeAMFCapacity *ngapType.RelativeAMFCapacity
		plmnSupportList     *ngapType.PLMNSupportList
	)

	for _, ie := range resp.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFName:
			amfName = ie.Value.AMFName
		case ngapType.ProtocolIEIDServedGUAMIList:
			guamiList = ie.Value.ServedGUAMIList
		case ngapType.ProtocolIEIDRelativeAMFCapacity:
			relativeAMFCapacity = ie.Value.RelativeAMFCapacity
		case ngapType.ProtocolIEIDPLMNSupportList:
			plmnSupportList = ie.Value.PLMNSupportList
		default:
			return fmt.Errorf("NGSetupResponse IE ID (%d) not supported", ie.Id.Value)
		}
	}

	if amfName == nil {
		return fmt.Errorf("AMF Name missing")
	}

	if amfName.Value != "amf" {
		return fmt.Errorf("AMF Name: got %q, want %q", amfName.Value, "amf")
	}

	if guamiList == nil {
		return fmt.Errorf("GUAMI List missing")
	}

	if relativeAMFCapacity == nil {
		return fmt.Errorf("relative AMF Capacity missing")
	}

	if plmnSupportList == nil {
		return fmt.Errorf("PLMN Support List missing")
	}

	if len(plmnSupportList.List) != 1 {
		return fmt.Errorf("PLMN Support List: got %d entries, want 1", len(plmnSupportList.List))
	}

	mcc, mnc := plmnIDToString(plmnSupportList.List[0].PLMNIdentity)
	if mcc != expMCC {
		return fmt.Errorf("MCC: got %q, want %q", mcc, expMCC)
	}

	if mnc != expMNC {
		return fmt.Errorf("MNC: got %q, want %q", mnc, expMNC)
	}

	if len(plmnSupportList.List[0].SliceSupportList.List) != 1 {
		return fmt.Errorf("slice support list: got %d entries, want 1", len(plmnSupportList.List[0].SliceSupportList.List))
	}

	sst, sd := snssaiToString(plmnSupportList.List[0].SliceSupportList.List[0].SNSSAI)
	if int(sst) != expSST {
		return fmt.Errorf("SST: got %d, want %d", sst, expSST)
	}

	if sd != expSD {
		return fmt.Errorf("SD: got %q, want %q", sd, expSD)
	}

	return nil
}

func plmnIDToString(ngapPlmnID ngapType.PLMNIdentity) (string, string) {
	hexString := strings.Split(hex.EncodeToString(ngapPlmnID.Value), "")
	mcc := hexString[1] + hexString[0] + hexString[3]

	var mnc string
	if hexString[2] == "f" {
		mnc = hexString[5] + hexString[4]
	} else {
		mnc = hexString[2] + hexString[5] + hexString[4]
	}

	return mcc, mnc
}

func snssaiToString(ngapSnssai ngapType.SNSSAI) (int32, string) {
	sst := int32(ngapSnssai.SST.Value[0])
	sd := ""

	if ngapSnssai.SD != nil {
		sd = hex.EncodeToString(ngapSnssai.SD.Value)
	}

	return sst, sd
}
