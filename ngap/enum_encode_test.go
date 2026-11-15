// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type enumCase struct {
	name  string
	root  int64
	known int64
	value func(int64) interface {
		per.Marshaler
		Name() string
		Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
	}
	decode func([]byte) (int64, error)
}

func enumCaseOf[T per.Integer, PT interface {
	*T
	per.Unmarshaler
}](name string, e *per.Enum[T], value func(int64) interface {
	per.Marshaler
	Name() string
	Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
},
) enumCase {
	return enumCase{
		name:  name,
		root:  int64(len(e.Root)),
		known: int64(len(e.Root) + len(e.Extension)),
		value: value,
		decode: func(b []byte) (int64, error) {
			v := PT(new(T))
			err := v.UnmarshalPER(per.NewReader(b), per.Aligned)

			return int64(*v), err
		},
	}
}

func extensibleEnums() []enumCase {
	return []enumCase{
		enumCaseOf("PagingDRX", &enumPagingDRX, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PagingDRX(v)
		}),
		enumCaseOf("TimeToWait", &enumTimeToWait, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return TimeToWait(v)
		}),
		enumCaseOf("RRCEstablishmentCause", &enumRRCEstablishmentCause, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RRCEstablishmentCause(v)
		}),
		enumCaseOf("TraceDepth", &enumTraceDepth, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return TraceDepth(v)
		}),
		enumCaseOf("NotificationCause", &enumNotificationCause, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NotificationCause(v)
		}),
		enumCaseOf("GUAMIType", &enumGUAMIType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return GUAMIType(v)
		}),
		enumCaseOf("HandoverType", &enumHandoverType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return HandoverType(v)
		}),
		enumCaseOf("EventType", &enumEventType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EventType(v)
		}),
	}
}

func TestEnumValuesRoundTripAsIndices(t *testing.T) {
	for _, tc := range append(extensibleEnums(), moreExtensibleEnums()...) {
		t.Run(tc.name, func(t *testing.T) {
			for _, v := range []int64{0, tc.root - 1, tc.root, tc.known - 1, tc.known, tc.known + 1, tc.root + 100} {
				if v < 0 {
					continue
				}

				b, err := per.Marshal(tc.value(v), per.Aligned)
				if err != nil {
					t.Fatalf("encode %d: %v", v, err)
				}

				want := extensionEnum(t, tc.root, v-tc.root)
				if v < tc.root {
					w := per.NewWriter()
					if err := per.EncodeEnumerated(w, per.Aligned, tc.root, true, v); err != nil {
						t.Fatal(err)
					}

					w.AlignToByte()
					want = w.Bytes()
				}

				if !bytes.Equal(b, want) {
					t.Fatalf("encode %d = %x, want the X.691 index encoding %x", v, b, want)
				}

				got, err := tc.decode(b)
				if err != nil || got != v {
					t.Fatalf("decode %x = %d, %v; want %d", b, got, err, v)
				}

				unknown := v >= tc.known
				if diagnosed := tc.value(v).Diagnose(&ap.Diagnosis[ProtocolIEID]{}); diagnosed != unknown {
					t.Errorf("value %d: Diagnose = %v, want %v", v, diagnosed, unknown)
				}

				if name := tc.value(v).Name(); unknown != (name == fmt.Sprintf("unknown(%d)", v)) {
					t.Errorf("value %d: Name = %q", v, name)
				}
			}
		})
	}
}
