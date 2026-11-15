// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// The k'th extension addition of an ENUMERATED whose root holds nRoot values.
func extensionEnum(t *testing.T, nRoot, k int64) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := per.EncodeEnumerated(w, per.Aligned, nRoot, true, nRoot+k); err != nil {
		t.Fatalf("encode extension %d: %v", k, err)
	}

	w.AlignToByte()

	return w.Bytes()
}

// An unguarded decoder narrows nRoot+k back onto a root value: a PagingOrigin
// extension with k=255 reads as 0 (non-3GPP), a PagingPriority one as 7
// (priolevel8). §10.3.1 case 6 handles it on criticality instead.
func TestEnumExtensionIsNotComprehended(t *testing.T) {
	tests := []struct {
		name    string
		nRoot   int64
		decode  func(*per.Reader) (int64, bool, error)
		aliases string
	}{
		{"PagingDRX", int64(len(enumPagingDRX.Root)), func(r *per.Reader) (int64, bool, error) {
			var v PagingDRX

			err := v.UnmarshalPER(r, per.Aligned)

			return int64(v), v.Diagnose(nil), err
		}, "a root paging cycle"},
		{"UERetentionInformation", int64(len(enumUERetentionInformation.Root)), func(r *per.Reader) (int64, bool, error) {
			var v UERetentionInformation

			err := v.UnmarshalPER(r, per.Aligned)

			return int64(v), v.Diagnose(nil), err
		}, "ues-retained"},
		{"TimeToWait", int64(len(enumTimeToWait.Root)), func(r *per.Reader) (int64, bool, error) {
			var v TimeToWait

			err := v.UnmarshalPER(r, per.Aligned)

			return int64(v), v.Diagnose(nil), err
		}, "a root wait time"},
		{"TimerApproachForGUAMIRemoval", int64(len(enumTimerApproachForGUAMIRemoval.Root)), func(r *per.Reader) (int64, bool, error) {
			var v TimerApproachForGUAMIRemoval

			err := v.UnmarshalPER(r, per.Aligned)

			return int64(v), v.Diagnose(nil), err
		}, "apply-timer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Spans a same-byte extension and one needing the normally-small
			// length form.
			for _, k := range []int64{0, 1, 64} {
				raw := extensionEnum(t, tt.nRoot, k)

				got, unknown, err := tt.decode(per.NewReader(raw))
				if err != nil {
					t.Fatalf("extension %d: decode is lenient, got %v", k, err)
				}

				if got != tt.nRoot+k || !unknown {
					t.Fatalf("extension %d decoded as %d (unknown %v), want index %d reported, not read as %s", k, got, unknown, tt.nRoot+k, tt.aliases)
				}
			}
		})
	}
}

// Through the IE-container engine: both are optional-ignore, so §10.3.4.2
// leaves the field absent and acts on the rest.
func TestPagingEnumExtensionsAreIgnored(t *testing.T) {
	tests := []struct {
		name    string
		id      ProtocolIEID
		nRoot   int64
		present func(*Paging) bool
	}{
		{"PagingPriority", IDPagingPriority, int64(len(enumPagingPriority.Root)), func(m *Paging) bool { return m.PagingPriority != nil }},
		{"PagingOrigin", IDPagingOrigin, int64(len(enumPagingOrigin.Root)), func(m *Paging) bool { return m.PagingOrigin != nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParsePaging(container(t,
				ieField{ID: IDPagingDRX, Criticality: CriticalityIgnore, Value: PagingDRXv128},
				ieField{ID: tt.id, Criticality: CriticalityIgnore, Value: ap.RawValue(extensionEnum(t, tt.nRoot, 64))},
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

// §10.3.4.2: continue "as if the not comprehended IEs/IE groups were not
// received". Storing the scratch value before checking the error would deliver
// a zero that reads as DefaultPagingDRX v32.
func TestNotComprehendedIEIsNotDelivered(t *testing.T) {
	msg, err := ParseRANConfigurationUpdate(container(t,
		ieField{ID: IDRANNodeName, Criticality: CriticalityIgnore, Value: Name("ella-gnb")},
		ieField{ID: IDDefaultPagingDRX, Criticality: CriticalityIgnore, Value: ap.RawValue(extensionEnum(t, int64(len(enumPagingDRX.Root)), 64))},
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

// §10.3.1 case 6 for the tag-driven ENUMERATEDs, whose root count is inlined
// into per_gen.go rather than reached through a guarded UnmarshalPER. This
// says nothing about whether the constant matches the tag — the extension bit
// is written from the same constant the decoder checks, so the two agree by
// construction; TestTaggedEnumRootCountMatchesTag covers that.
func TestTaggedEnumExtensionIsNotComprehended(t *testing.T) {
	for _, tt := range []struct {
		name   string
		encode func(*per.Writer) error
		decode func(*per.Reader) (ap.Diagnosable[ProtocolIEID], error)
	}{
		{"SecurityResult/IntegrityProtectionResult", func(w *per.Writer) error {
			w.WriteBit(false)
			w.WriteBit(false)

			return per.EncodeEnumerated(w, per.Aligned, int64(len(enumIntegrityProtectionResult.Root)), true, int64(len(enumIntegrityProtectionResult.Root)))
		}, func(r *per.Reader) (ap.Diagnosable[ProtocolIEID], error) {
			var v SecurityResult

			err := v.UnmarshalPER(r, per.Aligned)

			return &v, err
		}},
		{"SecurityIndication/IntegrityProtectionIndication", func(w *per.Writer) error {
			w.WriteBit(false)
			w.WriteBit(false)
			w.WriteBit(false)

			return per.EncodeEnumerated(w, per.Aligned, int64(len(enumIntegrityProtectionIndication.Root)), true, int64(len(enumIntegrityProtectionIndication.Root)))
		}, func(r *per.Reader) (ap.Diagnosable[ProtocolIEID], error) {
			var v SecurityIndication

			err := v.UnmarshalPER(r, per.Aligned)

			return &v, err
		}},
		{"AllocationAndRetentionPriority/PreemptionCapability", func(w *per.Writer) error {
			w.WriteBit(false)
			w.WriteBit(false)

			if err := PriorityLevelARP(1).MarshalPER(w, per.Aligned); err != nil {
				return err
			}

			return per.EncodeEnumerated(w, per.Aligned, int64(len(enumPreemptionCapability.Root)), true, int64(len(enumPreemptionCapability.Root)))
		}, func(r *per.Reader) (ap.Diagnosable[ProtocolIEID], error) {
			var v AllocationAndRetentionPriority

			err := v.UnmarshalPER(r, per.Aligned)

			return &v, err
		}},
		{"AssociatedQosFlowItem/QosFlowMappingIndication", func(w *per.Writer) error {
			w.WriteBit(false)
			w.WriteBit(true)
			w.WriteBit(false)

			if err := QosFlowIdentifier(3).MarshalPER(w, per.Aligned); err != nil {
				return err
			}

			return per.EncodeEnumerated(w, per.Aligned, int64(len(enumQosFlowMappingIndication.Root)), true, int64(len(enumQosFlowMappingIndication.Root)))
		}, func(r *per.Reader) (ap.Diagnosable[ProtocolIEID], error) {
			var v AssociatedQosFlowItem

			err := v.UnmarshalPER(r, per.Aligned)

			return &v, err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tt.encode(w); err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()

			v, err := tt.decode(per.NewReader(w.Bytes()))
			if err != nil {
				t.Fatalf("decode is lenient, got %v", err)
			}

			if !v.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
				t.Error("extension value read as a root value instead of reported not comprehended")
			}
		})
	}
}

// Encodes the last root value from the constant and requires the generated
// decoder to return it, which catches a tag whose width differs from the
// constant's. Only where the widths actually differ: the SecurityResult case
// (root 2, one bit) catches a widening to 3, but the SecurityIndication case
// (root 3) cannot catch a widening to 4, because X.691 §11.5.7.1 gives both
// two bits. TestQosEnumRootCountsMatchSpec is what pins that one.
func TestTaggedEnumRootCountMatchesTag(t *testing.T) {
	for _, tt := range []struct {
		name  string
		nRoot int64
		round func(*testing.T, int64)
	}{
		{"SecurityResult/IntegrityProtectionResult", int64(len(enumIntegrityProtectionResult.Root)), func(t *testing.T, last int64) {
			w := per.NewWriter()
			w.WriteBit(false)
			w.WriteBit(false)

			if err := per.EncodeEnumerated(w, per.Aligned, last+1, true, last); err != nil {
				t.Fatal(err)
			}

			if err := per.EncodeEnumerated(w, per.Aligned, int64(len(enumConfidentialityProtectionResult.Root)), true, 0); err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()

			var v SecurityResult
			if err := v.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
				t.Fatalf("last root value did not decode: %v", err)
			}

			if int64(v.IntegrityProtectionResult) != last {
				t.Errorf("IntegrityProtectionResult = %d, want %d — tag width disagrees with the RootCount constant", v.IntegrityProtectionResult, last)
			}
		}},
		{"SecurityIndication/IntegrityProtectionIndication", int64(len(enumIntegrityProtectionIndication.Root)), func(t *testing.T, last int64) {
			w := per.NewWriter()
			w.WriteBit(false)
			w.WriteBit(false)
			w.WriteBit(false)

			if err := per.EncodeEnumerated(w, per.Aligned, last+1, true, last); err != nil {
				t.Fatal(err)
			}

			if err := per.EncodeEnumerated(w, per.Aligned, int64(len(enumConfidentialityProtectionIndication.Root)), true, 0); err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()

			var v SecurityIndication
			if err := v.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
				t.Fatalf("last root value did not decode: %v", err)
			}

			if int64(v.IntegrityProtectionIndication) != last {
				t.Errorf("IntegrityProtectionIndication = %d, want %d — tag width disagrees with the RootCount constant", v.IntegrityProtectionIndication, last)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.round(t, tt.nRoot-1)
		})
	}
}
