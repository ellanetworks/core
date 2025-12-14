package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

func securityMode(ctx ctxt.Context, ue *context.AmfUe) error {
	ctx, span := tracer.Start(ctx, "securityMode")
	defer span.End()

	ue.Log = ue.Log.With(zap.String("supi", ue.Supi))
	ue.Log.Debug("EntryEvent at GMM State[SecurityMode]")

	if ue.SecurityContextIsValid() {
		ue.Log.Debug("UE has a valid security context - skip security mode control procedure")
		ue.State.Set(context.ContextSetup)
		return contextSetup(ctx, ue, ue.RegistrationRequest)
	}

	// Select enc/int algorithm based on ue security capability & amf's policy,
	amfSelf := context.AMFSelf()
	ue.SelectSecurityAlg(amfSelf.SecurityAlgorithm.IntegrityOrder, amfSelf.SecurityAlgorithm.CipheringOrder)
	// Generate KnasEnc, KnasInt
	ue.DerivateAlgKey()

	if ue.CipheringAlg == security.AlgCiphering128NEA0 && ue.IntegrityAlg == security.AlgIntegrity128NIA0 {
		ue.State.Set(context.ContextSetup)
		return nil
	}

	err := message.SendSecurityModeCommand(ctx, ue.RanUe)
	if err != nil {
		return fmt.Errorf("error sending security mode command: %v", err)
	}

	return nil
}
