// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// ExampleParseMessage decodes a plain EPS message and dispatches on its concrete
// type. The direction is a parameter because TS 24.301 table 9.8.1 gives DETACH
// REQUEST one message type in both directions; every other message decodes the
// same either way.
func ExampleParseMessage() {
	pdu := []byte{0x07, 0x44, 0x03}

	msg, err := eps.ParseMessage(pdu, nas.DirectionDownlink)
	if err != nil && !nas.SoftOnly(err) {
		fmt.Println("decode failed:", err)
		return
	}

	switch m := msg.(type) {
	case *eps.AttachReject:
		fmt.Println("attach rejected:", m.Cause)
	case *eps.AttachAccept:
		fmt.Println("attach accepted")
	default:
		fmt.Printf("unhandled %T\n", m)
	}

	// Output: attach rejected: Illegal UE (3)
}

// Example_buildAndProtect builds a message from a struct literal, encodes it, and
// wraps it in the security-protected message a peer with a security context
// expects. The NAS COUNT comes from a [nas.DownlinkCounter], which refuses to
// hand out the same count twice.
func Example_buildAndProtect() {
	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    nas.IntegrityAES,
		Ciphering:    nas.CipheringAES,
		IntegrityKey: nas.IntegrityKey{0x01, 0x02, 0x03},
		CipherKey:    nas.CipherKey{0x04, 0x05, 0x06},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	plain, err := (&eps.AttachReject{Cause: eps.EMMCauseIllegalUE}).MarshalBinary()
	if err != nil {
		fmt.Println(err)
		return
	}

	var counter nas.DownlinkCounter

	count, err := counter.Use()
	if err != nil {
		fmt.Println(err)
		return
	}

	wrapped, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, count, nas.DirectionDownlink, sc)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("plain %d octets, protected %d octets, sequence number %d\n",
		len(plain), len(wrapped), wrapped[5])

	// Output: plain 3 octets, protected 9 octets, sequence number 0
}

// Example_softErrors shows the soft-error contract: an optional element that does
// not decode leaves its field absent and is reported, but the rest of the message
// is usable, and the element survives re-encoding.
func Example_softErrors() {
	// An ATTACH REJECT whose T3402 value (IEI 0x16) declares two octets, where
	// TS 24.301 §9.9.3.16A gives the GPRS timer 2 exactly one.
	pdu := []byte{0x07, 0x44, 0x03, 0x16, 0x02, 0x21, 0x21}

	msg, err := eps.ParseMessage(pdu, nas.DirectionDownlink)
	if err == nil || !nas.SoftOnly(err) {
		fmt.Println("expected a soft error, got:", err)
		return
	}

	reject, ok := msg.(*eps.AttachReject)
	if !ok {
		fmt.Printf("unexpected %T\n", msg)
		return
	}

	fmt.Println("usable message, T3402 absent:", reject.T3402 == nil)

	for _, ie := range nas.IEErrors(err) {
		fmt.Printf("element %#02x did not decode\n", ie.IEI)
	}

	again, err := reject.MarshalBinary()
	fmt.Printf("re-encoded % x (err %v)\n", again, err)

	// Output:
	// usable message, T3402 absent: true
	// element 0x16 did not decode
	// re-encoded 07 44 03 16 02 21 21 (err <nil>)
}

// Example_receiveProtected verifies a security-protected message and commits the
// uplink NAS COUNT only once the MAC has verified, which is what stops a replay
// from advancing the count of a genuine subscriber.
func Example_receiveProtected() {
	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    nas.IntegrityAES,
		Ciphering:    nas.CipheringAES,
		IntegrityKey: nas.IntegrityKey{0x01, 0x02, 0x03},
		CipherKey:    nas.CipherKey{0x04, 0x05, 0x06},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	plain, err := (&eps.AttachComplete{ESMMessageContainer: []byte{0x02, 0x01, 0xC6}}).MarshalBinary()
	if err != nil {
		fmt.Println(err)
		return
	}

	wrapped, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, 0), nas.DirectionUplink, sc)
	if err != nil {
		fmt.Println(err)
		return
	}

	var counter nas.UplinkCounter

	spm, err := eps.ParseSecurityProtectedMessage(wrapped)
	if err != nil {
		fmt.Println(err)
		return
	}

	count := counter.Estimate(spm.SequenceNumber)

	body, sht, err := eps.Unprotect(wrapped, count, nas.DirectionUplink, sc,
		eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		fmt.Println("rejected:", err)
		return
	}

	if err := counter.Commit(count); err != nil {
		fmt.Println(err)
		return
	}

	msg, err := eps.ParseMessage(body, nas.DirectionUplink)
	if err != nil && !nas.SoftOnly(err) {
		fmt.Println(err)
		return
	}

	fmt.Printf("%T verified under %s\n", msg, sht)

	// A replay of the same octets estimates to the next expected count, not the
	// one it was sent under, so its MAC cannot verify.
	replayed := counter.Estimate(spm.SequenceNumber)

	if _, _, err := eps.Unprotect(wrapped, replayed, nas.DirectionUplink, sc); errors.Is(err, nas.ErrMACMismatch) {
		fmt.Println("replay refused")
	}

	// Output:
	// *eps.AttachComplete verified under integrity protected and ciphered
	// replay refused
}
