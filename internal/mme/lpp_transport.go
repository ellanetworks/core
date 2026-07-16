// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// TransferLPPToUE sends an LPP positioning message to a UE attached over E-UTRAN
// inside a DOWNLINK GENERIC NAS TRANSPORT (TS 24.301 §8.2.20). It is the LTE
// twin of the AMF's N1 transport: correlationID (nil when absent) travels in the
// Additional information IE so the UE echoes it and the reply routes back to the
// originating LCS session.
func (m *MME) TransferLPPToUE(ctx context.Context, supi etsi.SUPI, correlationID, lppMsg []byte) error {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("no UE context for %s", supi.String())
	}

	conn := ue.Conn()
	if conn == nil {
		return fmt.Errorf("UE %s is not connected", supi.String())
	}

	msg := &eps.DLGenericNASTransport{
		ContainerType:  eps.GenericContainerTypeLPP,
		Container:      lppMsg,
		AdditionalInfo: correlationID,
	}

	wire, err := ue.ProtectDownlinkMessage(msg)
	if err != nil {
		return fmt.Errorf("protect DL Generic NAS Transport (LPP): %w", err)
	}

	conn.SendDownlinkNASTransport(ctx, wire)

	logger.From(ctx, logger.MmeLog).Info("sent DL Generic NAS Transport (LPP) to UE",
		zap.String("supi", supi.String()),
		zap.Int("lpp_len", len(lppMsg)),
		zap.String("lpp_hex", hex.EncodeToString(lppMsg)),
		zap.String("lcs_correlation_id", hex.EncodeToString(correlationID)),
	)

	return nil
}
