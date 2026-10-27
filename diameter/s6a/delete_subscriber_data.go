// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type SubscriptionDataDeletion struct {
	Flags              DSRFlags
	SCEFID             string
	ContextIdentifiers []uint32
	TraceReference     []byte
	TSCodes            [][]byte
	SSCodes            [][]byte
}

type DeleteSubscriberDataRequest struct {
	IMSI     string
	Features Features
	SubscriptionDataDeletion
}

type DeleteSubscriberData struct {
	Features Features
	Flags    DSAFlags
}

var (
	deletionRules = diameter.Rules{
		vendorKey(AVPDSRFlags):          {Required: true, MinLength: 4},
		vendorKey(AVPSCEFID):            {},
		vendorKey(AVPContextIdentifier): {Multiple: true},
		vendorKey(AVPTraceReference):    {},
		vendorKey(AVPTSCode):            {Multiple: true},
		vendorKey(AVPSSCode):            {Multiple: true},
	}
	subscriptionDataDeletionRules = deletionRules
	dsrRules                      = requestRules(true, userNameRule.With(deletionRules).With(diameter.Rules{
		vendorKey(AVPEDRXRelatedRAT):     {},
		vendorKey(AVPExternalIdentifier): {Multiple: true},
	}))
)

func (d SubscriptionDataDeletion) contextsWithdrawn() bool {
	return d.Flags&dsrPDNAndPDPContextsWithdrawal != 0
}

func (d SubscriptionDataDeletion) avps() ([]diameter.AVP, error) {
	switch {
	case !d.Flags.Known():
		return nil, invalidf("DSR-Flags %s: PDN and PDP context withdrawal are exclusive", d.Flags)
	case d.Flags&DSRFlagDeleteMonitoringEvents != 0 && d.SCEFID == "":
		return nil, invalidf("monitoring event deletion without an SCEF-ID")
	case d.contextsWithdrawn() && len(d.ContextIdentifiers) == 0:
		return nil, invalidf("context withdrawal without a Context-Identifier")
	case !d.contextsWithdrawn() && len(d.ContextIdentifiers) > 0:
		return nil, invalidf("Context-Identifier without a context withdrawal")
	case d.Flags&DSRFlagTraceDataWithdrawal == 0 && len(d.TraceReference) > 0:
		return nil, invalidf("Trace-Reference without trace data withdrawal")
	case d.Flags&DSRFlagSMSWithdrawal == 0 && len(d.TSCodes) > 0:
		return nil, invalidf("TS-Code without SMS withdrawal")
	case d.Flags&(DSRFlagSMSWithdrawal|DSRFlagLCSWithdrawal) == 0 && len(d.SSCodes) > 0:
		return nil, invalidf("SS-Code without SMS or LCS withdrawal")
	}

	avps := []diameter.AVP{unsignedAVP(AVPDSRFlags, uint32(d.Flags))}
	avps = appendString(avps, AVPSCEFID, d.SCEFID)

	for _, id := range d.ContextIdentifiers {
		if id == 0 {
			return nil, invalidf("zero context identifier")
		}

		avps = append(avps, unsignedAVP(AVPContextIdentifier, id))
	}

	if len(d.TraceReference) > 0 {
		avps = append(avps, octetsAVP(AVPTraceReference, d.TraceReference))
	}

	for _, code := range d.TSCodes {
		avps = append(avps, octetsAVP(AVPTSCode, code))
	}

	for _, code := range d.SSCodes {
		avps = append(avps, octetsAVP(AVPSSCode, code))
	}

	return avps, nil
}

func parseSubscriptionDataDeletion(avps []diameter.AVP) (SubscriptionDataDeletion, error) {
	var d SubscriptionDataDeletion

	flags, ok := diameter.Find(avps, AVPDSRFlags, tgpp.VendorID)
	if !ok {
		return SubscriptionDataDeletion{}, tgpp.MissingAVP(AVPDSRFlags, tgpp.VendorID, 4)
	}

	v, err := tgpp.Unsigned32(flags)
	if err != nil {
		return SubscriptionDataDeletion{}, err
	}

	if d.Flags = DSRFlags(v); !d.Flags.Known() {
		return SubscriptionDataDeletion{}, tgpp.InvalidAVP(flags)
	}

	if d.SCEFID, err = optionalString(avps, AVPSCEFID, tgpp.VendorID); err != nil {
		return SubscriptionDataDeletion{}, err
	}

	if d.Flags&DSRFlagDeleteMonitoringEvents != 0 && d.SCEFID == "" {
		return SubscriptionDataDeletion{}, tgpp.MissingAVP(AVPSCEFID, tgpp.VendorID, 0)
	}

	if d.contextsWithdrawn() {
		ids := diameter.FindAll(avps, AVPContextIdentifier, tgpp.VendorID)
		if len(ids) == 0 {
			return SubscriptionDataDeletion{}, tgpp.MissingAVP(AVPContextIdentifier, tgpp.VendorID, 4)
		}

		for _, a := range ids {
			id, err := tgpp.Unsigned32(a)
			if err != nil {
				return SubscriptionDataDeletion{}, err
			}

			if id == 0 {
				return SubscriptionDataDeletion{}, tgpp.InvalidAVP(a)
			}

			d.ContextIdentifiers = append(d.ContextIdentifiers, id)
		}
	}

	if a, ok := diameter.Find(avps, AVPTraceReference, tgpp.VendorID); ok && d.Flags&DSRFlagTraceDataWithdrawal != 0 && len(a.Data) > 0 {
		d.TraceReference = bytes.Clone(a.Data)
	}

	if d.Flags&DSRFlagSMSWithdrawal != 0 {
		d.TSCodes = octetsList(avps, AVPTSCode)
	}

	if d.Flags&(DSRFlagSMSWithdrawal|DSRFlagLCSWithdrawal) != 0 {
		d.SSCodes = octetsList(avps, AVPSSCode)
	}

	return d, nil
}

func octetsList(avps []diameter.AVP, code uint32) [][]byte {
	var out [][]byte

	for _, a := range diameter.FindAll(avps, code, tgpp.VendorID) {
		out = append(out, bytes.Clone(a.Data))
	}

	return out
}

func NewDeleteSubscriberDataRequest(env tgpp.Envelope, r DeleteSubscriberDataRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	deletion, err := r.avps()
	if err != nil {
		return nil, err
	}

	avps := append([]diameter.AVP{userName}, r.Features.avps()...)

	return newRequest(env, CommandDeleteSubscriberData, true, append(avps, deletion...)...)
}

func CheckDeleteSubscriberData(req *diameter.Message) error {
	return dsrRules.Check(req)
}

func ParseDeleteSubscriberDataRequest(req *diameter.Message) (DeleteSubscriberDataRequest, error) {
	if err := CheckDeleteSubscriberData(req); err != nil {
		return DeleteSubscriberDataRequest{}, err
	}

	imsi, err := requestIMSI(req)
	if err != nil {
		return DeleteSubscriberDataRequest{}, err
	}

	d, err := parseSubscriptionDataDeletion(req.AVPs)
	if err != nil {
		return DeleteSubscriberDataRequest{}, err
	}

	return DeleteSubscriberDataRequest{IMSI: imsi, Features: featuresOf(req), SubscriptionDataDeletion: d}, nil
}

func NewDeleteSubscriberDataAnswer(req *diameter.Message, id diameter.Identity, d DeleteSubscriberData) (*diameter.Message, error) {
	ans := newSuccessAnswer(req, id, d.Features)

	var err error

	if ans.AVPs, err = appendFlags(ans.AVPs, AVPDSAFlags, "DSA-Flags", d.Flags); err != nil {
		return nil, err
	}

	return ans, nil
}

func ParseDeleteSubscriberDataAnswer(ans *diameter.Message) (DeleteSubscriberData, error) {
	features, err := parseAnswer(ans)
	if err != nil {
		return DeleteSubscriberData{}, err
	}

	d := DeleteSubscriberData{Features: features}

	if d.Flags, err = answerFlags[DSAFlags](ans.AVPs, AVPDSAFlags); err != nil {
		return DeleteSubscriberData{}, malformed(err)
	}

	return d, nil
}
