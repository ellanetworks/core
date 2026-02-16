package gnb

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"golang.org/x/sync/errgroup"
)

const NumRadios = 24

type NGSetupResponse struct{}

func (NGSetupResponse) Meta() engine.Meta {
	return engine.Meta{
		ID:      "gnb/ngap/setup_response",
		Summary: "NGSetup request/response test validating the NGSetupResponse message contents with 24 radios in parallel",
		Timeout: 2 * time.Second,
	}
}

func (t NGSetupResponse) Run(ctx context.Context, env engine.Env) error {
	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, getDefaultEllaCoreConfig())

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	eg := errgroup.Group{}

	for i := range NumRadios {
		func() {
			eg.Go(func() error {
				return ngSetupTest(env, i)
			})
		}()
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("NGSetupResponse test failed: %v", err)
	}

	// Cleanup
	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}

func ngSetupTest(env engine.Env, index int) error {
	gNodeB, err := gnb.Start(
		fmt.Sprintf("%06x", index+1),
		DefaultMCC,
		DefaultMNC,
		DefaultSST,
		DefaultSD,
		DefaultDNN,
		DefaultTAC,
		fmt.Sprintf("Ella-Core-Tester-%d", index),
		env.Config.EllaCore.N2Address,
		env.Config.Gnb.N2Address,
		"",
	)
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	nextFrame, err := gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	err = utils.ValidateSCTP(nextFrame.Info, 60, 0)
	if err != nil {
		return fmt.Errorf("SCTP validation failed: %v", err)
	}

	pdu, err := ngap.Decoder(nextFrame.Data)
	if err != nil {
		return fmt.Errorf("could not decode NGAP: %v", err)
	}

	if pdu.SuccessfulOutcome == nil {
		return fmt.Errorf("NGAP PDU is not a SuccessfulOutcome")
	}

	if pdu.SuccessfulOutcome.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup {
		return fmt.Errorf("NGAP ProcedureCode is not NGSetup (%d)", ngapType.ProcedureCodeNGSetup)
	}

	nGSetupResponse := pdu.SuccessfulOutcome.Value.NGSetupResponse
	if nGSetupResponse == nil {
		return fmt.Errorf("NGSetupResponse is nil")
	}

	err = validateNGSetupResponse(nGSetupResponse, &NGSetupResponseValidationOpts{
		MCC: DefaultMCC,
		MNC: DefaultMNC,
		SST: DefaultSST,
		SD:  DefaultSD,
	})
	if err != nil {
		return fmt.Errorf("NGSetupResponse validation failed: %v", err)
	}

	return nil
}

type NGSetupResponseValidationOpts struct {
	MCC string
	MNC string
	SST int32
	SD  string
}

func validateNGSetupResponse(nGSetupResponse *ngapType.NGSetupResponse, opts *NGSetupResponseValidationOpts) error {
	var (
		amfName             *ngapType.AMFName
		guamiList           *ngapType.ServedGUAMIList
		relativeAMFCapacity *ngapType.RelativeAMFCapacity
		plmnSupportList     *ngapType.PLMNSupportList
	)

	for _, ie := range nGSetupResponse.ProtocolIEs.List {
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
		return fmt.Errorf("AMF Name is missing in NGSetupResponse")
	}

	if amfName.Value != "amf" {
		return fmt.Errorf("AMF Name value is incorrect, got: %s, want: amf", amfName.Value)
	}

	if guamiList == nil {
		return fmt.Errorf("served GUAMI List is missing in NGSetupResponse")
	}

	if relativeAMFCapacity == nil {
		return fmt.Errorf("relative AMF Capacity is missing in NGSetupResponse")
	}

	if plmnSupportList == nil {
		return fmt.Errorf("PLMN Support List is missing in NGSetupResponse")
	}

	// check plmnSupportList has exactly one item
	if len(plmnSupportList.List) != 1 {
		return fmt.Errorf("PLMN Support List should have exactly one item, got: %d", len(plmnSupportList.List))
	}

	mcc, mnc := plmnIDToString(plmnSupportList.List[0].PLMNIdentity)
	if mcc != opts.MCC {
		return fmt.Errorf("PLMN Identity MCC is incorrect, got: %s, want: %s", mcc, opts.MCC)
	}

	if mnc != opts.MNC {
		return fmt.Errorf("PLMN Identity MNC is incorrect, got: %s, want: %s", mnc, opts.MNC)
	}

	if len(plmnSupportList.List[0].SliceSupportList.List) != 1 {
		return fmt.Errorf("slice support list should have exactly one item, got: %d", len(plmnSupportList.List[0].SliceSupportList.List))
	}

	sst, sd := snssaiToString(plmnSupportList.List[0].SliceSupportList.List[0].SNSSAI)
	if sst != opts.SST {
		return fmt.Errorf("SST is incorrect, got: %v, want: %v", sst, opts.SST)
	}

	if sd != opts.SD {
		return fmt.Errorf("SD is incorrect, got: %s, want: %s", sd, opts.SD)
	}

	return nil
}

func plmnIDToString(ngapPlmnID ngapType.PLMNIdentity) (string, string) {
	value := ngapPlmnID.Value
	hexString := strings.Split(hex.EncodeToString(value), "")
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
