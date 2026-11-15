// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

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
		enumCaseOf("ForbiddenInterRATs", &enumForbiddenInterRATs, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ForbiddenInterRATs(v)
		}),
		enumCaseOf("GWContextReleaseIndication", &enumGWContextReleaseIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return GWContextReleaseIndication(v)
		}),
		enumCaseOf("CauseRadioNetwork", &enumCauseRadioNetwork, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseRadioNetwork(v)
		}),
		enumCaseOf("CauseTransport", &enumCauseTransport, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseTransport(v)
		}),
		enumCaseOf("CauseNas", &enumCauseNas, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseNas(v)
		}),
		enumCaseOf("CauseProtocol", &enumCauseProtocol, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseProtocol(v)
		}),
		enumCaseOf("CauseMisc", &enumCauseMisc, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseMisc(v)
		}),
		enumCaseOf("HandoverType", &enumHandoverType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return HandoverType(v)
		}),
		enumCaseOf("GUMMEIType", &enumGUMMEIType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return GUMMEIType(v)
		}),
		enumCaseOf("NRrestrictioninEPSasSecondaryRAT", &enumNRrestrictioninEPSasSecondaryRAT, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NRrestrictioninEPSasSecondaryRAT(v)
		}),
		enumCaseOf("UnlicensedSpectrumRestriction", &enumUnlicensedSpectrumRestriction, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UnlicensedSpectrumRestriction(v)
		}),
		enumCaseOf("NRrestrictionin5GS", &enumNRrestrictionin5GS, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NRrestrictionin5GS(v)
		}),
		enumCaseOf("CNType", &enumCNType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CNType(v)
		}),
		enumCaseOf("RequestTypeAdditionalInfo", &enumRequestTypeAdditionalInfo, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RequestTypeAdditionalInfo(v)
		}),
		enumCaseOf("DAPSIndicator", &enumDAPSIndicator, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DAPSIndicator(v)
		}),
		enumCaseOf("IMSvoiceEPSfallbackfrom5G", &enumIMSvoiceEPSfallbackfrom5G, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IMSvoiceEPSfallbackfrom5G(v)
		}),
		enumCaseOf("EmergencyIndicator", &enumEmergencyIndicator, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EmergencyIndicator(v)
		}),
		enumCaseOf("SubcarrierSpacingSSB", &enumSubcarrierSpacingSSB, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SubcarrierSpacingSSB(v)
		}),
		enumCaseOf("RATType", &enumRATType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RATType(v)
		}),
		enumCaseOf("CoarseUELocationRequested", &enumCoarseUELocationRequested, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CoarseUELocationRequested(v)
		}),
		enumCaseOf("TimeRefDistribution", &enumTimeRefDistribution, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return TimeRefDistribution(v)
		}),
		enumCaseOf("SONInformationRequest", &enumSONInformationRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SONInformationRequest(v)
		}),
		enumCaseOf("SynchronisationStatus", &enumSynchronisationStatus, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SynchronisationStatus(v)
		}),
		enumCaseOf("MutingAvailabilityIndication", &enumMutingAvailabilityIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MutingAvailabilityIndication(v)
		}),
		enumCaseOf("MutingPatternPeriod", &enumMutingPatternPeriod, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MutingPatternPeriod(v)
		}),
		enumCaseOf("PatternPeriod", &enumPatternPeriod, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PatternPeriod(v)
		}),
	}
}

func TestEnumValuesRoundTripAsIndices(t *testing.T) {
	for _, tc := range extensibleEnums() {
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
