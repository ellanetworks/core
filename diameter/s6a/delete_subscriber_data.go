// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type DeleteSubscriberDataRequest struct {
	IMSI               string
	Features           Features
	Flags              DSRFlags
	SCEFID             string
	ContextIdentifiers []uint32
	TraceReference     []byte
	TSCodes            [][]byte
	SSCodes            [][]byte
}

type DeleteSubscriberData struct {
	Features Features
	Flags    DSAFlags
}

var dsrRules = requestRules(true, userNameRule.With(diameter.Rules{
	vendorKey(AVPDSRFlags):           {Required: true, MinLength: 4},
	vendorKey(AVPSCEFID):             {},
	vendorKey(AVPContextIdentifier):  {Multiple: true},
	vendorKey(AVPTraceReference):     {},
	vendorKey(AVPTSCode):             {Multiple: true},
	vendorKey(AVPSSCode):             {Multiple: true},
	vendorKey(AVPEDRXRelatedRAT):     {},
	vendorKey(AVPExternalIdentifier): {Multiple: true},
}))

func NewDeleteSubscriberDataRequest(env tgpp.Envelope, r DeleteSubscriberDataRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	if r.Flags&DSRFlagDeleteMonitoringEvents != 0 && r.SCEFID == "" {
		return nil, invalidf("monitoring event deletion without an SCEF-ID")
	}

	avps := append([]diameter.AVP{userName}, r.Features.avps()...)
	avps = append(avps, unsignedAVP(AVPDSRFlags, uint32(r.Flags)))

	if r.SCEFID != "" {
		avps = append(avps, diameter.UTF8String(AVPSCEFID, diameter.AVPFlagMandatory, tgpp.VendorID, r.SCEFID))
	}

	for _, id := range r.ContextIdentifiers {
		if id == 0 {
			return nil, invalidf("zero context identifier")
		}

		avps = append(avps, unsignedAVP(AVPContextIdentifier, id))
	}

	if len(r.TraceReference) > 0 {
		avps = append(avps, octetsAVP(AVPTraceReference, r.TraceReference))
	}

	for _, code := range r.TSCodes {
		avps = append(avps, octetsAVP(AVPTSCode, code))
	}

	for _, code := range r.SSCodes {
		avps = append(avps, octetsAVP(AVPSSCode, code))
	}

	return newRequest(env, CommandDeleteSubscriberData, true, avps...)
}

func CheckDeleteSubscriberData(req *diameter.Message) error {
	if err := dsrRules.Check(req); err != nil {
		return err
	}

	flags, _ := req.Find(AVPDSRFlags, tgpp.VendorID)

	v, err := tgpp.Unsigned32(flags)
	if err != nil {
		return err
	}

	if _, ok := req.Find(AVPSCEFID, tgpp.VendorID); DSRFlags(v)&DSRFlagDeleteMonitoringEvents != 0 && !ok {
		return tgpp.MissingAVP(AVPSCEFID, tgpp.VendorID, 0)
	}

	return nil
}

func ParseDeleteSubscriberDataRequest(req *diameter.Message) (DeleteSubscriberDataRequest, error) {
	if err := CheckDeleteSubscriberData(req); err != nil {
		return DeleteSubscriberDataRequest{}, err
	}

	var (
		r   DeleteSubscriberDataRequest
		err error
	)

	if r.IMSI, err = requestIMSI(req); err != nil {
		return DeleteSubscriberDataRequest{}, err
	}

	r.Features = featuresOf(req)

	flags, _ := req.Find(AVPDSRFlags, tgpp.VendorID)
	v, _ := flags.Unsigned32()
	r.Flags = DSRFlags(v)

	if r.SCEFID, err = optionalString(req.AVPs, AVPSCEFID, tgpp.VendorID); err != nil {
		return DeleteSubscriberDataRequest{}, err
	}

	for _, a := range diameter.FindAll(req.AVPs, AVPContextIdentifier, tgpp.VendorID) {
		id, err := tgpp.Unsigned32(a)
		if err != nil {
			return DeleteSubscriberDataRequest{}, err
		}

		if id == 0 {
			return DeleteSubscriberDataRequest{}, tgpp.InvalidAVP(a)
		}

		r.ContextIdentifiers = append(r.ContextIdentifiers, id)
	}

	if a, ok := req.Find(AVPTraceReference, tgpp.VendorID); ok {
		r.TraceReference = bytes.Clone(a.Data)
	}

	for _, a := range diameter.FindAll(req.AVPs, AVPTSCode, tgpp.VendorID) {
		r.TSCodes = append(r.TSCodes, bytes.Clone(a.Data))
	}

	for _, a := range diameter.FindAll(req.AVPs, AVPSSCode, tgpp.VendorID) {
		r.SSCodes = append(r.SSCodes, bytes.Clone(a.Data))
	}

	return r, nil
}

func NewDeleteSubscriberDataAnswer(req *diameter.Message, id diameter.Identity, d DeleteSubscriberData) (*diameter.Message, error) {
	ans := newSuccessAnswer(req, id, d.Features)

	var err error

	if ans.AVPs, err = appendFlags(ans.AVPs, AVPDSAFlags, "DSA-Flags", d.Flags, dsaFlagsDefined); err != nil {
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

	if d.Flags, err = optionalFlags(ans.AVPs, AVPDSAFlags, dsaFlagsDefined); err != nil {
		return DeleteSubscriberData{}, malformed(err)
	}

	return d, nil
}
