// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

const (
	goldenHandoverNotify = "000b4022000003000a000200010055000200020079400f4002f839000000010002f839000001"
	// NotifySourceNGRANNode present: id 269 (0x010d), ignore, one root value.
	goldenHandoverNotifyNotifySource = "000b4027000004000a000200010055000200020079400f4002f839000000010002f8390000010" +
		"10d400100"
)

func handoverNotifyULI() *UserLocationInformation {
	plmn := PLMNIdentity{0x02, 0xf8, 0x39}

	return &UserLocationInformation{
		Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x000000010,
		TAI: TAI{PLMNIdentity: plmn, TAC: 1},
	}
}

func TestHandoverNotifyGolden(t *testing.T) {
	notifySource := NotifySourceNGRANNodeNotifySource

	for _, c := range []struct {
		name   string
		in     HandoverNotify
		golden string
	}{
		{"Min", HandoverNotify{
			AMFUENGAPID: 1, RANUENGAPID: 2, UserLocationInformation: handoverNotifyULI(),
		}, goldenHandoverNotify},
		{"NotifySource", HandoverNotify{
			AMFUENGAPID: 1, RANUENGAPID: 2, UserLocationInformation: handoverNotifyULI(),
			NotifySourceNGRANNode: &notifySource,
		}, goldenHandoverNotifyNotifySource},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.in.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(b); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			im, ok := pdu.(*InitiatingMessage)
			if !ok || im.ProcedureCode != ProcHandoverNotification {
				t.Fatalf("decoded %T with procedure %v, want an initiating HandoverNotification", pdu, im)
			}

			out, err := ParseHandoverNotify(im.Value)
			if err != nil {
				t.Fatal(err)
			}

			if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 {
				t.Errorf("UE NGAP IDs = %d/%d, want 1/2", out.AMFUENGAPID, out.RANUENGAPID)
			}

			if out.UserLocationInformation == nil || out.UserLocationInformation.Kind != UserLocationNR ||
				out.UserLocationInformation.CellIdentity != 0x000000010 {
				t.Errorf("UserLocationInformation = %+v, want the NR cell", out.UserLocationInformation)
			}

			if (c.in.NotifySourceNGRANNode == nil) != (out.NotifySourceNGRANNode == nil) {
				t.Errorf("NotifySourceNGRANNode presence not round-tripped")
			}
		})
	}
}

// UserLocationInformation is mandatory with ignore criticality, so §10.3.5
// leaves the procedure to continue without it rather than rejecting; the AMF
// still has to see that it was missing.
func TestHandoverNotifyMissingUserLocation(t *testing.T) {
	value := container(t,
		ieField{id: IDAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: IDRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	out, err := ParseHandoverNotify(value)
	if err != nil {
		t.Fatalf("an absent ignore-criticality IE must not reject the procedure: %v", err)
	}

	if out.UserLocationInformation != nil {
		t.Errorf("UserLocationInformation = %+v, want nil", out.UserLocationInformation)
	}

	ies := out.meta().diagnostics.IEs
	if len(ies) != 1 || ies[0].ID != IDUserLocationInformation || ies[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for UserLocationInformation", ies)
	}
}

// A missing mandatory reject IE rejects the procedure (§10.3.5).
func TestHandoverNotifyMissingAMFUENGAPID(t *testing.T) {
	value := container(t,
		ieField{id: IDRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	if _, err := ParseHandoverNotify(value); err == nil {
		t.Fatal("parsed without the AMF UE NGAP ID")
	} else {
		var ase *AbstractSyntaxError
		if !errors.As(err, &ase) {
			t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
		}
	}
}

// NotifySourceNGRANNode has a single root value, so index 1 is the first
// extension and must be refused rather than read as notifySource.
func TestNotifySourceNGRANNodeExtensionIsNotComprehended(t *testing.T) {
	raw := extensionEnum(t, notifySourceNGRANNodeRootCount, 0)

	var v NotifySourceNGRANNode

	err := v.UnmarshalPER(per.NewReader(raw), per.Aligned)
	if err == nil {
		t.Fatal("extension value decoded as notifySource")
	}

	if !errors.Is(err, errNotComprehended) {
		t.Errorf("err = %v, want errNotComprehended so criticality decides", err)
	}
}

// Root counts 1 and 2 both encode index 0 as a zero octet, so no golden
// separates them. The value is pinned against the ASN.1 instead.
func TestNotifySourceNGRANNodeRootCountMatchesSpec(t *testing.T) {
	if notifySourceNGRANNodeRootCount != 1 {
		t.Errorf("root count is %d, TS 38.413 defines one member before the extension marker",
			notifySourceNGRANNodeRootCount)
	}
}
