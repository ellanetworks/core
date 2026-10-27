// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/models"
)

var (
	sdPattern        = regexp.MustCompile(`^[A-Fa-f0-9]{6}$`)
	snssaiKeyPattern = regexp.MustCompile(`^(\d{1,3})(?:-([A-Fa-f0-9]{6}))?$`)
)

type Snssai struct {
	Sst *int   `json:"sst"`
	Sd  string `json:"sd,omitempty"`
}

func NewSnssai(sst int, sd string) Snssai {
	return Snssai{Sst: &sst, Sd: sd}
}

func (s Snssai) Check(Policy) error {
	return firstError(
		checkRequiredRange("/sst", s.Sst, 0, 255),
		checkPattern("/sd", s.Sd, sdPattern, true),
	)
}

func (s Snssai) Equal(o Snssai) bool {
	return s.Sst != nil && o.Sst != nil && *s.Sst == *o.Sst && strings.EqualFold(s.Sd, o.Sd)
}

func (s Snssai) String() string {
	if s.Sst == nil {
		return ""
	}

	if s.Sd == "" {
		return strconv.Itoa(*s.Sst)
	}

	return strconv.Itoa(*s.Sst) + "-" + strings.ToLower(s.Sd)
}

func ParseSnssai(key string) (Snssai, error) {
	m := snssaiKeyPattern.FindStringSubmatch(key)
	if m == nil {
		return Snssai{}, fmt.Errorf("wire: S-NSSAI key %q is not <sst>[-<sd>]", key)
	}

	sst, _ := strconv.Atoi(m[1])
	if sst > 255 {
		return Snssai{}, fmt.Errorf("wire: S-NSSAI key %q: SST out of range", key)
	}

	return NewSnssai(sst, strings.ToLower(m[2])), nil
}

func SnssaiFromModel(m models.Snssai) Snssai {
	sd := ""
	if m.Sd != "" {
		sd = models.NormalizeSD(m.Sd)
	}

	return NewSnssai(int(m.Sst), sd)
}

func (s Snssai) Model() (models.Snssai, error) {
	if err := s.Check(Strict); err != nil {
		return models.Snssai{}, err
	}

	return models.Snssai{Sst: int32(*s.Sst), Sd: strings.ToLower(s.Sd)}, nil
}
