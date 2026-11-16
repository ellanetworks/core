// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"
)

func TestNGSetupResponseGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenNGSetupResponse))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcNGSetup {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	resp, err := ParseNGSetupResponse(so.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if resp.AMFName != "ella-amf" {
		t.Errorf("AMFName = %q", resp.AMFName)
	}

	if len(resp.ServedGUAMIList) != 1 {
		t.Fatalf("ServedGUAMIList = %+v", resp.ServedGUAMIList)
	}

	want := goldResponse().ServedGUAMIList[0].GUAMI
	if got := resp.ServedGUAMIList[0].GUAMI; got != want {
		t.Errorf("GUAMI = %+v, want %+v", got, want)
	}

	if resp.ServedGUAMIList[0].BackupAMFName != nil {
		t.Errorf("BackupAMFName = %q, want nil", deref(resp.ServedGUAMIList[0].BackupAMFName))
	}

	if deref(resp.RelativeAMFCapacity) != 255 {
		t.Errorf("RelativeAMFCapacity = %d", deref(resp.RelativeAMFCapacity))
	}

	if len(resp.PLMNSupportList) != 1 || resp.PLMNSupportList[0].PLMNIdentity != goldPLMN() {
		t.Errorf("PLMNSupportList = %+v", resp.PLMNSupportList)
	}
}

// A response re-encoded from a decode must be byte-identical, or the AMF's
// answer changes shape each time it passes through this codec.
func TestNGSetupResponseReencodeStable(t *testing.T) {
	b1, err := goldResponse().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b1)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseNGSetupResponse(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	b2, err := out.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if string(b1) != string(b2) {
		t.Fatalf("re-encode unstable:\n  b1 % x\n  b2 % x", b1, b2)
	}
}

func goldNGSetupResponseExtendedAMFName() *NGSetupResponse {
	m := goldResponse()
	m.ServedGUAMIList[0].ExtendedBackupAMFName = &ExtendedAMFName{
		AMFNameVisibleString: Ptr(AMFNameVisibleString("ella-amf-backup-extended")),
	}
	m.ExtendedAMFName = &ExtendedAMFName{
		AMFNameVisibleString: Ptr(AMFNameVisibleString("ella-amf-extended")),
		AMFNameUTF8String:    Ptr(AMFNameUTF8String("ella-amf-ü")),
	}

	return m
}

func nGSetupResponseGoldenCases() []goldenRoundTrip {
	return []goldenRoundTrip{
		{
			"NGSetupResponseExtendedAMFName",
			goldNGSetupResponseExtendedAMFName(),
			func(v []byte) (any, error) { return ParseNGSetupResponse(v) },
			"201500740000050001000a0380656c6c612d616d6600600028002002f839020043000001ba401a40b8656c6c612d616d662d6261636b75702d657874656e64656400564001ff0050000b0002f839000010080102030112401f6080656c6c612d616d662d657874656e6465640b656c6c612d616d662dc3bc",
		},
	}
}

func TestNGSetupResponseExtendedAMFNameGolden(t *testing.T) {
	for _, c := range nGSetupResponseGoldenCases() {
		t.Run(c.name, func(t *testing.T) { checkGoldenRoundTrip(t, c) })
	}
}
