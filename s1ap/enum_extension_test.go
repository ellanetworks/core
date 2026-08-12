// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

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

// An unguarded decoder narrows nRoot+k back onto a root value: a PagingPriority
// extension with k=255 reads as 7 (priolevel8). §10.3.1 case 6 handles it on
// criticality instead.
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
		{"TimeToWait", timeToWaitRootCount, func(r *per.Reader) error {
			var v TimeToWait
			return v.UnmarshalPER(r, per.Aligned)
		}, "a root wait time"},
		{"RRCEstablishmentCause", rrcEstablishmentCauseRootCount, func(r *per.Reader) error {
			var v RRCEstablishmentCause
			return v.UnmarshalPER(r, per.Aligned)
		}, "a root establishment cause"},
		// HandoverType is absent deliberately: its first two extension additions
		// are values TS 36.413 §9.2.1.13 assigns and Ella decodes.
		// TestHandoverTypeInterSystemValues covers both those and the additions
		// beyond them, which are refused the same way.
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

// Through the IE-container engine: the IE is optional-ignore, so §10.3.4.2
// leaves the field absent and acts on the rest.
func TestPagingEnumExtensionsAreIgnored(t *testing.T) {
	msg, err := ParsePaging(container(t,
		ieField{id: IDPagingDRX, crit: CriticalityIgnore, val: PagingDRXv128},
		ieField{id: IDPagingPriority, crit: CriticalityIgnore, raw: extensionEnum(t, pagingPriorityRootCount, 255)},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.PagingPriority != nil {
		t.Errorf("PagingPriority = %v, decoded from an extension value; want it left absent", *msg.PagingPriority)
	}

	if msg.PagingDRX == nil {
		t.Error("an ignore-criticality IE took a comprehended sibling with it")
	}
}

// §10.3.4.2: continue "as if the not comprehended IEs/IE groups were not
// received". Storing the scratch value before checking the error would deliver
// a zero that reads as DefaultPagingDRX v32.
func TestNotComprehendedIEIsNotDelivered(t *testing.T) {
	msg, err := ParseENBConfigurationUpdate(container(t,
		ieField{id: IDENBname, crit: CriticalityIgnore, val: Name("ella-enb")},
		ieField{id: IDDefaultPagingDRX, crit: CriticalityIgnore, raw: extensionEnum(t, pagingDRXRootCount, 255)},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.DefaultPagingDRX != nil {
		t.Errorf("DefaultPagingDRX = %v, want nil: the IE was not comprehended", *msg.DefaultPagingDRX)
	}

	if deref(msg.ENBName) != "ella-enb" {
		t.Errorf("ENBName = %q, want the comprehended IE still delivered", deref(msg.ENBName))
	}
}
