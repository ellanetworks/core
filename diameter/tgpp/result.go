// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter"
)

var ErrMalformedResult = errors.New("tgpp: malformed result")

type Result struct {
	Code         uint32
	Experimental bool
	VendorID     uint32
}

func Experimental(code uint32) Result {
	return Result{Code: code, Experimental: true, VendorID: VendorID}
}

func (r Result) Success() bool {
	return r.Code >= 2000 && r.Code < 3000
}

func (r Result) IsExperimental(code uint32) bool {
	return r.Experimental && r.VendorID == VendorID && r.Code == code
}

func (r Result) DiameterResult() Result {
	return r
}

func (r Result) String() string {
	if r.Experimental {
		return fmt.Sprintf("experimental result %d (vendor %d)", r.Code, r.VendorID)
	}

	return fmt.Sprintf("result %d", r.Code)
}

type ResultError interface {
	error
	DiameterResult() Result
}

func ResultOf(err error) (Result, bool) {
	var re ResultError
	if !errors.As(err, &re) {
		return Result{}, false
	}

	return re.DiameterResult(), true
}

func IsExperimental(err error, code uint32) bool {
	r, ok := ResultOf(err)

	return ok && r.IsExperimental(code)
}

func ParseResult(ans *diameter.Message) (Result, error) {
	if rc, ok := ans.Find(diameter.AVPResultCode, 0); ok {
		code, err := rc.Unsigned32()
		if err != nil {
			return Result{}, fmt.Errorf("%w: Result-Code", ErrMalformedResult)
		}

		return Result{Code: code}, nil
	}

	er, ok := ans.Find(diameter.AVPExperimentalResult, 0)
	if !ok {
		return Result{}, fmt.Errorf("%w: no Result-Code or Experimental-Result", ErrMalformedResult)
	}

	inner, err := er.Grouped()
	if err != nil {
		return Result{}, fmt.Errorf("%w: Experimental-Result", ErrMalformedResult)
	}

	vendorAVP, ok := diameter.Find(inner, diameter.AVPVendorID, 0)
	if !ok {
		return Result{}, fmt.Errorf("%w: no Vendor-Id in Experimental-Result", ErrMalformedResult)
	}

	vendorID, err := vendorAVP.Unsigned32()
	if err != nil {
		return Result{}, fmt.Errorf("%w: Experimental-Result Vendor-Id", ErrMalformedResult)
	}

	codeAVP, ok := diameter.Find(inner, diameter.AVPExperimentalResultCode, 0)
	if !ok {
		return Result{}, fmt.Errorf("%w: no Experimental-Result-Code", ErrMalformedResult)
	}

	code, err := codeAVP.Unsigned32()
	if err != nil {
		return Result{}, fmt.Errorf("%w: Experimental-Result-Code", ErrMalformedResult)
	}

	return Result{Code: code, Experimental: true, VendorID: vendorID}, nil
}
