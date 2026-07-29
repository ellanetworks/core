// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs_test

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// ExampleParseMessage decodes a plain 5GS message and dispatches on its concrete
// type. The type switch is complete: a message type the package does not model
// arrives as an [fgs.UnknownMessage].
func ExampleParseMessage() {
	pdu := []byte{0x7e, 0x00, 0x44, 0x0b}

	msg, err := fgs.ParseMessage(pdu)
	if err != nil && !nas.SoftOnly(err) {
		fmt.Println("decode failed:", err)
		return
	}

	switch m := msg.(type) {
	case *fgs.RegistrationReject:
		fmt.Println("registration rejected:", m.Cause)
	case *fgs.RegistrationAccept:
		fmt.Println("registration accepted")
	default:
		fmt.Printf("unhandled %T\n", m)
	}

	// Output: registration rejected: PLMN not allowed (11)
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

	cause := fgs.GMMCausePLMNNotAllowed

	plain, err := (&fgs.RegistrationReject{Cause: cause}).MarshalBinary()
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

	wrapped, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedCiphered, count, nas.DirectionDownlink, sc)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("plain %d octets, protected %d octets, sequence number %d\n",
		len(plain), len(wrapped), wrapped[6])

	// Output: plain 4 octets, protected 11 octets, sequence number 0
}

// Example_softErrors shows the soft-error contract: an optional element that does
// not decode leaves its field absent and is reported, but the rest of the message
// is usable, and the element survives re-encoding.
func Example_softErrors() {
	// A REGISTRATION ACCEPT whose T3512 value (IEI 0x5E) declares two octets,
	// where TS 24.501 §9.11.2.5 gives the GPRS timer 3 exactly one.
	pdu := []byte{0x7e, 0x00, 0x42, 0x01, 0x01, 0x5e, 0x02, 0x21, 0x21}

	msg, err := fgs.ParseMessage(pdu)
	if err == nil || !nas.SoftOnly(err) {
		fmt.Println("expected a soft error, got:", err)
		return
	}

	accept, ok := msg.(*fgs.RegistrationAccept)
	if !ok {
		fmt.Printf("unexpected %T\n", msg)
		return
	}

	fmt.Println("usable message, T3512 absent:", accept.T3512 == nil)

	for _, ie := range nas.IEErrors(err) {
		fmt.Printf("element %#02x did not decode\n", ie.IEI)
	}

	again, err := accept.MarshalBinary()
	fmt.Printf("re-encoded % x (err %v)\n", again, err)

	// Output:
	// usable message, T3512 absent: true
	// element 0x5e did not decode
	// re-encoded 7e 00 42 01 01 5e 02 21 21 (err <nil>)
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

	plain, err := (&fgs.RegistrationComplete{}).MarshalBinary()
	if err != nil {
		fmt.Println(err)
		return
	}

	wrapped, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedCiphered, nas.MakeCount(0, 0), nas.DirectionUplink, sc)
	if err != nil {
		fmt.Println(err)
		return
	}

	var counter nas.UplinkCounter

	spm, err := fgs.ParseSecurityProtectedMessage(wrapped)
	if err != nil {
		fmt.Println(err)
		return
	}

	count := counter.Estimate(spm.SequenceNumber)

	body, sht, err := fgs.Unprotect(wrapped, count, nas.DirectionUplink, sc,
		fgs.SHTIntegrityProtectedCiphered)
	if err != nil {
		fmt.Println("rejected:", err)
		return
	}

	if err := counter.Commit(count); err != nil {
		fmt.Println(err)
		return
	}

	msg, err := fgs.ParseMessage(body)
	if err != nil && !nas.SoftOnly(err) {
		fmt.Println(err)
		return
	}

	fmt.Printf("%T verified under %s\n", msg, sht)

	// A replay of the same octets estimates to the next expected count, not the
	// one it was sent under, so its MAC cannot verify.
	replayed := counter.Estimate(spm.SequenceNumber)

	if _, _, err := fgs.Unprotect(wrapped, replayed, nas.DirectionUplink, sc); errors.Is(err, nas.ErrMACMismatch) {
		fmt.Println("replay refused")
	}

	// Output:
	// *fgs.RegistrationComplete verified under integrity protected and ciphered
	// replay refused
}
