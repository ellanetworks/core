// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package osutil

import (
	"sync"
	"syscall"
)

var umaskMu sync.Mutex

func WithTightUmask(fn func() error) error {
	umaskMu.Lock()
	defer umaskMu.Unlock()

	prev := syscall.Umask(0o077)
	defer syscall.Umask(prev)

	return fn()
}
