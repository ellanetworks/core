// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const (
	QueryPlmnID            = "plmn-id"
	QuerySingleNssai       = "single-nssai"
	QueryDnn               = "dnn"
	QueryDatasetNames      = "dataset-names"
	QuerySupportedFeatures = "supported-features"
)

func queryError(name string, err error) error {
	return &IEError{Param: "query " + name, Reason: err.Error()}
}

func jsonQueryValue(name string, v Checker) (string, error) {
	if err := v.Check(Strict); err != nil {
		return "", queryError(name, err)
	}

	b, err := json.Marshal(v)
	if err != nil {
		return "", queryError(name, err)
	}

	return string(b), nil
}

func PlmnIDQueryValue(p PlmnID) (string, error) {
	return jsonQueryValue(QueryPlmnID, p)
}

func SnssaiQueryValue(s Snssai) (string, error) {
	return jsonQueryValue(QuerySingleNssai, s)
}

func ParsePlmnIDQueryValue(v string) (PlmnID, error) {
	var p PlmnID
	if err := json.Unmarshal([]byte(v), &p); err != nil {
		return PlmnID{}, queryError(QueryPlmnID, err)
	}

	if err := p.Check(Strict); err != nil {
		return PlmnID{}, queryError(QueryPlmnID, err)
	}

	return p, nil
}

func ParseSnssaiQueryValue(v string) (Snssai, error) {
	var s Snssai
	if err := json.Unmarshal([]byte(v), &s); err != nil {
		return Snssai{}, queryError(QuerySingleNssai, err)
	}

	if err := s.Check(Strict); err != nil {
		return Snssai{}, queryError(QuerySingleNssai, err)
	}

	return s, nil
}

func DatasetNamesQueryValue(names ...DataSetName) (string, error) {
	if len(names) < 2 {
		return "", queryError(QueryDatasetNames, fmt.Errorf("needs at least 2 data sets, got %d", len(names)))
	}

	seen := map[DataSetName]bool{}
	parts := make([]string, 0, len(names))

	for _, n := range names {
		if !n.Known() || seen[n] {
			return "", queryError(QueryDatasetNames, fmt.Errorf("invalid or repeated data set %q", n))
		}

		seen[n] = true

		parts = append(parts, string(n))
	}

	return strings.Join(parts, ","), nil
}

func ParseDatasetNamesQueryValue(v string) []DataSetName {
	var out []DataSetName

	for n := range strings.SplitSeq(v, ",") {
		if n != "" {
			out = append(out, DataSetName(n))
		}
	}

	return out
}

type SDMQuery struct {
	DatasetNames      []DataSetName
	PlmnID            *PlmnID
	SingleNssai       *Snssai
	Dnn               string
	SupportedFeatures SupportedFeatures
}

func (q SDMQuery) Values() (url.Values, error) {
	v := url.Values{}

	if len(q.DatasetNames) > 0 {
		s, err := DatasetNamesQueryValue(q.DatasetNames...)
		if err != nil {
			return nil, err
		}

		v.Set(QueryDatasetNames, s)
	}

	if q.PlmnID != nil {
		s, err := PlmnIDQueryValue(*q.PlmnID)
		if err != nil {
			return nil, err
		}

		v.Set(QueryPlmnID, s)
	}

	if q.SingleNssai != nil {
		s, err := SnssaiQueryValue(*q.SingleNssai)
		if err != nil {
			return nil, err
		}

		v.Set(QuerySingleNssai, s)
	}

	if q.Dnn != "" {
		if !ValidDnn(q.Dnn) {
			return nil, queryError(QueryDnn, fmt.Errorf("invalid DNN %q", q.Dnn))
		}

		v.Set(QueryDnn, q.Dnn)
	}

	if q.SupportedFeatures != "" {
		if err := q.SupportedFeatures.Check(Strict); err != nil {
			return nil, queryError(QuerySupportedFeatures, err)
		}

		v.Set(QuerySupportedFeatures, string(q.SupportedFeatures))
	}

	return v, nil
}
