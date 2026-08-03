// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// extensionEnum encodes the k'th extension addition of an extensible
// ENUMERATED whose root holds nRoot values.
func extensionEnum(t *testing.T, nRoot, k int64) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := per.EncodeEnumerated(w, per.Aligned, nRoot, true, nRoot+k); err != nil {
		t.Fatalf("encode extension %d: %v", k, err)
	}

	w.AlignToByte()

	return w.Bytes()
}

// A value this version does not know must not read as one it does.
// per.DecodeEnumerated reports the k'th extension as nRoot+k, and every
// enumeration here is a small unsigned Go type, so an unguarded decoder
// narrows nRoot+k straight back onto a root value: a PagingOrigin extension
// with k=255 would read as 0 (non-3GPP) and a PagingPriority extension with
// k=255 as 7 (priolevel8). TS 38.413 §10.3.1 case 6 makes an unsupported
// value an abstract syntax error handled on criticality instead.
func TestEnumExtensionIsNotComprehended(t *testing.T) {
	tests := []struct {
		name    string
		nRoot   int64
		decode  func(*per.Reader) error
		aliases string
	}{
		{"PagingDRX", pagingDRXRootCount, func(r *per.Reader) error {
			var v PagingDRX
			return v.UnmarshalPER(r, per.Aligned)
		}, "a root paging cycle"},
		{"UERetentionInformation", ueRetentionInformationRootCount, func(r *per.Reader) error {
			var v UERetentionInformation
			return v.UnmarshalPER(r, per.Aligned)
		}, "ues-retained"},
		{"TimeToWait", timeToWaitRootCount, func(r *per.Reader) error {
			var v TimeToWait
			return v.UnmarshalPER(r, per.Aligned)
		}, "a root wait time"},
		{"TimerApproachForGUAMIRemoval", timerApproachForGUAMIRemovalRootCount, func(r *per.Reader) error {
			var v TimerApproachForGUAMIRemoval
			return v.UnmarshalPER(r, per.Aligned)
		}, "apply-timer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// k spans a same-byte extension and one that needs the
			// normally-small length form.
			for _, k := range []int64{0, 1, 255} {
				raw := extensionEnum(t, tt.nRoot, k)

				err := tt.decode(per.NewReader(raw))
				if err == nil {
					t.Fatalf("extension %d decoded, want it refused rather than read as %s", k, tt.aliases)
				}

				if !errors.Is(err, errNotComprehended) {
					t.Errorf("extension %d: err = %v, want errNotComprehended so criticality decides (§10.3.1 case 6)", k, err)
				}
			}
		})
	}
}

// The same rule through the IE-container engine: both Paging enumerations are
// optional with ignore criticality, so an unknown value leaves the field absent
// and the rest of the message is still acted on (§10.3.4.2).
func TestPagingEnumExtensionsAreIgnored(t *testing.T) {
	tests := []struct {
		name    string
		id      ProtocolIEID
		nRoot   int64
		present func(*Paging) bool
	}{
		{"PagingPriority", idPagingPriority, pagingPriorityRootCount, func(m *Paging) bool { return m.PagingPriority != nil }},
		{"PagingOrigin", idPagingOrigin, pagingOriginRootCount, func(m *Paging) bool { return m.PagingOrigin != nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParsePaging(container(t,
				ieField{id: idPagingDRX, crit: CriticalityIgnore, val: PagingDRXv128},
				ieField{id: tt.id, crit: CriticalityIgnore, raw: extensionEnum(t, tt.nRoot, 255)},
			))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if tt.present(msg) {
				t.Errorf("%s decoded from an extension value, want it left absent", tt.name)
			}

			if msg.PagingDRX == nil {
				t.Error("an ignore-criticality IE took a comprehended sibling with it")
			}
		})
	}
}

// TS 38.413 §10.3.4.2 for an ignore-criticality IE: "continue with the
// procedure as if the not comprehended IEs/IE groups were not received". A
// decoder that stores its scratch value before checking the error leaves the
// caller a zero that reads as a real one — here DefaultPagingDRX v32, which
// would silently reset a gNB's paging cycle.
func TestNotComprehendedIEIsNotDelivered(t *testing.T) {
	msg, err := ParseRANConfigurationUpdate(container(t,
		ieField{id: idRANNodeName, crit: CriticalityIgnore, val: Name("ella-gnb")},
		ieField{id: idDefaultPagingDRX, crit: CriticalityIgnore, raw: extensionEnum(t, pagingDRXRootCount, 255)},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.DefaultPagingDRX != nil {
		t.Errorf("DefaultPagingDRX = %v, want nil: the IE was not comprehended", *msg.DefaultPagingDRX)
	}

	if deref(msg.RANNodeName) != "ella-gnb" {
		t.Errorf("RANNodeName = %q, want the comprehended IE still delivered", deref(msg.RANNodeName))
	}
}
