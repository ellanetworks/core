package utils

import (
	"encoding/binary"
	"fmt"

	"github.com/ishidawataru/sctp"
)

func ValidateSCTP(info *sctp.SndRcvInfo, expectedPPID uint32, expectedStreamID uint16) error {
	if info == nil {
		return fmt.Errorf("missing SCTP SndRcvInfo")
	}

	if info.PPID != nativeToNetworkEndianness32(expectedPPID) {
		return fmt.Errorf("ppid=%d want %d (NGAP)", info.PPID, nativeToNetworkEndianness32(expectedPPID))
	}

	if info.Stream != expectedStreamID {
		return fmt.Errorf("stream=%d want %d (non-UE signalling)", info.Stream, expectedStreamID)
	}

	return nil
}

func nativeToNetworkEndianness32(value uint32) uint32 {
	var b [4]byte

	binary.NativeEndian.PutUint32(b[:], value)

	return binary.BigEndian.Uint32(b[:])
}
