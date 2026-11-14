// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

type UETNLABindingReleaseRequest struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID

	messageMeta
}

var uETNLABindingReleaseRequestIEs = []ieSpec[UETNLABindingReleaseRequest]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UETNLABindingReleaseRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *UETNLABindingReleaseRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UETNLABindingReleaseRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *UETNLABindingReleaseRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
}

func (m *UETNLABindingReleaseRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUETNLABindingRelease, uETNLABindingReleaseRequestIEs, m)
}

func (m *UETNLABindingReleaseRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUETNLABindingRelease,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUETNLABindingReleaseRequest(value []byte) (*UETNLABindingReleaseRequest, error) {
	return parseMessageBody[UETNLABindingReleaseRequest](ProcUETNLABindingRelease, TriggeringInitiatingMessage, uETNLABindingReleaseRequestIEs, value)
}
