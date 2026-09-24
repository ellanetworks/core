// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

func TestS1APStreamForProcedureCoversEveryOutboundMessage(t *testing.T) {
	cases := map[S1APProcedure]uint16{
		S1APProcedureS1SetupResponse:                   S1apStreamNonUE,
		S1APProcedureS1SetupFailure:                    S1apStreamNonUE,
		S1APProcedurePaging:                            S1apStreamNonUE,
		S1APProcedureResetAcknowledge:                  S1apStreamNonUE,
		S1APProcedureErrorIndication:                   S1apStreamNonUE,
		S1APProcedureENBConfigUpdateAck:                S1apStreamNonUE,
		S1APProcedureENBConfigUpdateFailure:            S1apStreamNonUE,
		S1APProcedureMMEConfigUpdate:                   S1apStreamNonUE,
		S1APProcedureMMEConfigurationTransfer:          S1apStreamNonUE,
		S1APProcedureInitialContextSetupRequest:        S1apStreamUE,
		S1APProcedureUEContextReleaseCommand:           S1apStreamUE,
		S1APProcedureDownlinkNASTransport:              S1apStreamUE,
		S1APProcedureERABSetupRequest:                  S1apStreamUE,
		S1APProcedureERABModifyRequest:                 S1apStreamUE,
		S1APProcedureERABReleaseCommand:                S1apStreamUE,
		S1APProcedureERABModificationConfirm:           S1apStreamUE,
		S1APProcedureHandoverRequest:                   S1apStreamUE,
		S1APProcedureHandoverCommand:                   S1apStreamUE,
		S1APProcedureHandoverPreparationFailure:        S1apStreamUE,
		S1APProcedureHandoverCancelAcknowledge:         S1apStreamUE,
		S1APProcedureMMEStatusTransfer:                 S1apStreamUE,
		S1APProcedurePathSwitchRequestAck:              S1apStreamUE,
		S1APProcedurePathSwitchRequestFailure:          S1apStreamUE,
		S1APProcedureDownlinkUEAssociatedLPPaTransport: S1apStreamUE,
	}

	for p, want := range cases {
		got, err := s1apStreamForProcedure(p)
		if err != nil || got != want {
			t.Errorf("s1apStreamForProcedure(%s) = %d, %v, want %d", p, got, err, want)
		}
	}
}

func TestS1APStreamForProcedureRejectsAnInboundOnlyMessage(t *testing.T) {
	if _, err := s1apStreamForProcedure(S1APProcedureInitialUEMessage); err == nil {
		t.Fatal("s1apStreamForProcedure(InitialUEMessage) = nil error, want one for a message the MME never sends")
	}
}
