// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sctp

import (
	"testing"
	"unsafe"
)

// getAddrsOld is passed to the kernel as struct sctp_getaddrs_old, which
// rejects anything shorter and reads past anything longer.
func TestGetAddrsOldMatchesKernelLayout(t *testing.T) {
	var param getAddrsOld

	// sctp_assoc_t (4) + int (4) + a pointer aligned to its own width.
	want := 8 + unsafe.Sizeof(uintptr(0))
	if got := unsafe.Sizeof(param); got != want {
		t.Errorf("sizeof(getAddrsOld) = %d, want %d", got, want)
	}

	if got := unsafe.Offsetof(param.AddrNum); got != 4 {
		t.Errorf("offsetof(AddrNum) = %d, want 4", got)
	}

	if got := unsafe.Offsetof(param.Addrs); got != unsafe.Sizeof(uintptr(0)) {
		t.Errorf("offsetof(Addrs) = %d, want %d", got, unsafe.Sizeof(uintptr(0)))
	}
}

// These constants are kernel ABI (include/uapi/linux/sctp.h). Most are declared
// positionally with iota, so adding or removing a member silently renumbers the
// rest; the resulting setsockopt calls would target the wrong option.
func TestConstantsMatchKernelABI(t *testing.T) {
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"SCTP_RTOINFO", sctpOptRtoInfo, 0},
		{"SCTP_ASSOCINFO", sctpOptAssocInfo, 1},
		{"SCTP_INITMSG", sctpOptInitMsg, 2},
		{"SCTP_NODELAY", sctpOptNoDelay, 3},
		{"SCTP_EVENTS", sctpOptEvents, 11},
		{"SCTP_DELAYED_ACK_TIME", sctpOptDelayedAckTime, 16},
		{"SCTP_SOCKOPT_BINDX_ADD", sctpOptBindxAdd, 100},
		{"SCTP_GET_PEER_ADDRS", sctpOptGetPeerAddrs, 108},
		{"SCTP_GET_LOCAL_ADDRS", sctpOptGetLocalAddrs, 109},
		{"SCTP_SOCKOPT_CONNECTX3", sctpOptConnectX3, 111},
		{"SCTP_CMSG_INIT", sctpCMsgInit, 0},
		{"SCTP_CMSG_SNDRCV", sctpCMsgSndRcv, 1},
		{"SCTP_DATA_IO_EVENT", sctpEventDataIO, 1},
		{"SCTP_ASSOCIATION_EVENT", sctpEventAssociation, 2},
		{"SCTP_SHUTDOWN_EVENT", sctpEventShutdown, 32},
		{"SCTP_PARTIAL_DELIVERY_EVENT", sctpEventPartialDelivery, 64},
		{"SCTP_SN_TYPE_BASE", int(SCTPSnTypeBase), 0x8000},
		{"SCTP_ASSOC_CHANGE", int(SCTPAssocChange), 0x8001},
		{"SCTP_SHUTDOWN_EVENT notification", int(SCTPShutdownEvent), 0x8005},
		{"SCTP_PARTIAL_DELIVERY_EVENT notification", int(SCTPPartialDeliveryEvent), 0x8006},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}
