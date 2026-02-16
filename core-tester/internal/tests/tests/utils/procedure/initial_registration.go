package procedure

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

const timeoutPerMessage = 5 * time.Second

type InitialRegistrationOpts struct {
	RANUENGAPID int64
	UE          *ue.UE
}

func InitialRegistration(opts *InitialRegistrationOpts) (*nas.Message, error) {
	err := opts.UE.SendRegistrationRequest(opts.RANUENGAPID, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		return nil, fmt.Errorf("could not build Registration Request NAS PDU: %v", err)
	}

	_, err = opts.UE.WaitForNASGMMMessage(nas.MsgTypeRegistrationAccept, timeoutPerMessage)
	if err != nil {
		return nil, fmt.Errorf("did not receive Registration Accept after initial registration: %v", err)
	}

	msg, err := opts.UE.WaitForNASGSMMessage(nas.MsgTypePDUSessionEstablishmentAccept, timeoutPerMessage)
	if err != nil {
		return nil, fmt.Errorf("timeout waiting for PDU session establishment accept: %v", err)
	}

	_, err = opts.UE.WaitForPDUSession(timeoutPerMessage)
	if err != nil {
		return nil, fmt.Errorf("timeout waiting for PDU session: %v", err)
	}

	// Sleep to ensure gNodeB sends the PDU Session Resource Setup Response before proceeding
	time.Sleep(50 * time.Millisecond)

	return msg, nil
}
