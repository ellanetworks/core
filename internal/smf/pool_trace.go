// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// poolTrace returns a short call-stack string identifying who triggered an SMF
// session-pool mutation. TEMPORARY diagnostic for the 4G EPS-session lookup
// regression ("no EPS session for <imsi>"); remove once the root cause is fixed.
func poolTrace() string {
	pcs := make([]uintptr, 10)
	n := runtime.Callers(2, pcs) // skip runtime.Callers + poolTrace itself
	frames := runtime.CallersFrames(pcs[:n])

	var b strings.Builder

	for i := 0; i < 9; i++ {
		f, more := frames.Next()
		if f.Function == "" {
			break
		}

		fn := f.Function
		if idx := strings.LastIndex(fn, "."); idx >= 0 {
			fn = fn[idx+1:]
		}

		if i > 0 {
			b.WriteString(" <- ")
		}

		fmt.Fprintf(&b, "%s(%s:%d)", fn, filepath.Base(f.File), f.Line)

		if !more {
			break
		}
	}

	return b.String()
}
