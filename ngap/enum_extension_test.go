// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
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
			// Spans a same-byte extension and one needing the normally-small
			// length form.
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

// Through the IE-container engine: both are optional-ignore, so §10.3.4.2
// leaves the field absent and acts on the rest.
func TestPagingEnumExtensionsAreIgnored(t *testing.T) {
	tests := []struct {
		name    string
		id      ProtocolIEID
		nRoot   int64
		present func(*Paging) bool
	}{
		{"PagingPriority", IDPagingPriority, pagingPriorityRootCount, func(m *Paging) bool { return m.PagingPriority != nil }},
		{"PagingOrigin", IDPagingOrigin, pagingOriginRootCount, func(m *Paging) bool { return m.PagingOrigin != nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParsePaging(container(t,
				ieField{id: IDPagingDRX, crit: CriticalityIgnore, val: PagingDRXv128},
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

// §10.3.4.2: continue "as if the not comprehended IEs/IE groups were not
// received". Storing the scratch value before checking the error would deliver
// a zero that reads as DefaultPagingDRX v32.
func TestNotComprehendedIEIsNotDelivered(t *testing.T) {
	msg, err := ParseRANConfigurationUpdate(container(t,
		ieField{id: IDRANNodeName, crit: CriticalityIgnore, val: Name("ella-gnb")},
		ieField{id: IDDefaultPagingDRX, crit: CriticalityIgnore, raw: extensionEnum(t, pagingDRXRootCount, 255)},
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
		decode func(*per.Reader) error
	}{
		{"SecurityResult/IntegrityProtectionResult", func(w *per.Writer) error {
			w.WriteBit(false)
			w.WriteBit(false)

			return per.EncodeEnumerated(w, per.Aligned, integrityProtectionResultRootCount, true, integrityProtectionResultRootCount)
		}, func(r *per.Reader) error {
			var v SecurityResult
			return v.UnmarshalPER(r, per.Aligned)
		}},
		{"SecurityIndication/IntegrityProtectionIndication", func(w *per.Writer) error {
			w.WriteBit(false)
			w.WriteBit(false)
			w.WriteBit(false)

			return per.EncodeEnumerated(w, per.Aligned, integrityProtectionIndicationRootCount, true, integrityProtectionIndicationRootCount)
		}, func(r *per.Reader) error {
			var v SecurityIndication
			return v.UnmarshalPER(r, per.Aligned)
		}},
		{"AllocationAndRetentionPriority/PreemptionCapability", func(w *per.Writer) error {
			w.WriteBit(false)
			w.WriteBit(false)

			if err := PriorityLevelARP(1).MarshalPER(w, per.Aligned); err != nil {
				return err
			}

			return per.EncodeEnumerated(w, per.Aligned, preemptionCapabilityRootCount, true, preemptionCapabilityRootCount)
		}, func(r *per.Reader) error {
			var v AllocationAndRetentionPriority
			return v.UnmarshalPER(r, per.Aligned)
		}},
		{"AssociatedQosFlowItem/QosFlowMappingIndication", func(w *per.Writer) error {
			w.WriteBit(false)
			w.WriteBit(true)
			w.WriteBit(false)

			if err := QosFlowIdentifier(3).MarshalPER(w, per.Aligned); err != nil {
				return err
			}

			return per.EncodeEnumerated(w, per.Aligned, qosFlowMappingIndicationRootCount, true, qosFlowMappingIndicationRootCount)
		}, func(r *per.Reader) error {
			var v AssociatedQosFlowItem
			return v.UnmarshalPER(r, per.Aligned)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tt.encode(w); err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()

			err := tt.decode(per.NewReader(w.Bytes()))
			if err == nil {
				t.Fatal("extension value decoded, want it refused rather than read as a root value")
			}

			if !errors.Is(err, errNotComprehended) {
				t.Errorf("err = %v, want errNotComprehended so criticality decides (§10.3.1 case 6)", err)
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
		{"SecurityResult/IntegrityProtectionResult", integrityProtectionResultRootCount, func(t *testing.T, last int64) {
			w := per.NewWriter()
			w.WriteBit(false)
			w.WriteBit(false)

			if err := per.EncodeEnumerated(w, per.Aligned, last+1, true, last); err != nil {
				t.Fatal(err)
			}

			if err := per.EncodeEnumerated(w, per.Aligned, confidentialityProtectionResultRootCount, true, 0); err != nil {
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
		{"SecurityIndication/IntegrityProtectionIndication", integrityProtectionIndicationRootCount, func(t *testing.T, last int64) {
			w := per.NewWriter()
			w.WriteBit(false)
			w.WriteBit(false)
			w.WriteBit(false)

			if err := per.EncodeEnumerated(w, per.Aligned, last+1, true, last); err != nil {
				t.Fatal(err)
			}

			if err := per.EncodeEnumerated(w, per.Aligned, confidentialityProtectionIndicationRootCount, true, 0); err != nil {
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
