// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas"
	"go.uber.org/zap"
)

// decoded reports whether a parse produced a message the AMF may act on, and
// logs the optional elements it had to drop.
//
// TS 24.501 §7.7.1 requires the network to treat a syntactically incorrect
// optional IE as not present rather than discard the message, so a soft error is
// not a reason to reject: the affected fields are absent and everything else
// decoded. A hard error — bad framing, a malformed mandatory element, or one of
// the security-critical elements — yields no message and is rejected.
func decoded(ctx context.Context, message string, err error) bool {
	if err == nil {
		return true
	}

	if !nas.SoftOnly(err) {
		return false
	}

	logger.From(ctx, logger.AmfLog).Warn("ignoring syntactically incorrect optional IEs (TS 24.501 §7.7.1)",
		zap.String("message", message), zap.Error(err))

	return true
}
