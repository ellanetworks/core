// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

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

// Through the IE-container engine: the IE is optional-ignore, so §10.3.4.2
// leaves the field absent and acts on the rest.
func TestPagingEnumExtensionsAreIgnored(t *testing.T) {
	msg, err := ParsePaging(container(t,
		ieField{ID: IDPagingDRX, Criticality: CriticalityIgnore, Value: PagingDRXv128},
		ieField{ID: IDPagingPriority, Criticality: CriticalityIgnore, Value: ap.RawValue(extensionEnum(t, int64(len(enumPagingPriority.Root)), 100))},
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
		ieField{ID: IDENBname, Criticality: CriticalityIgnore, Value: Name("ella-enb")},
		ieField{ID: IDDefaultPagingDRX, Criticality: CriticalityIgnore, Value: ap.RawValue(extensionEnum(t, int64(len(enumPagingDRX.Root)), 100))},
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
