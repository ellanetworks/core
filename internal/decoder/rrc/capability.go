// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

type NRBand struct {
	Band        int64  `json:"band"`
	PowerClass  string `json:"power_class,omitempty"`
	PUSCH256QAM bool   `json:"pusch_256qam,omitempty"`
	ChannelBWDL string `json:"channel_bw_dl,omitempty"`
	ChannelBWUL string `json:"channel_bw_ul,omitempty"`
}

type NRCapability struct {
	AccessStratumRelease string   `json:"access_stratum_release,omitempty"`
	Bands                []NRBand `json:"bands,omitempty"`
}

type EUTRABand struct {
	Band       int64 `json:"band"`
	HalfDuplex bool  `json:"half_duplex,omitempty"`
}

type EUTRACapability struct {
	AccessStratumRelease string      `json:"access_stratum_release,omitempty"`
	UECategory           int64       `json:"ue_category,omitempty"`
	Bands                []EUTRABand `json:"bands,omitempty"`
}

type Capability struct {
	NR    *NRCapability    `json:"nr,omitempty"`
	EUTRA *EUTRACapability `json:"eutra,omitempty"`
	Error string           `json:"error,omitempty"`
}

type ratContainer struct {
	ratType string
	payload []byte
}

func decodeWith(reg map[string]node, name string, b []byte) (any, error) {
	n, ok := reg[name]
	if !ok {
		return nil, fmt.Errorf("rrc: undefined type %q", name)
	}

	return n.decode(per.NewReader(b))
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}

func choiceValue(v any) any {
	for _, alt := range asMap(v) {
		return alt
	}

	return nil
}

func choiceKey(v any) string {
	for k := range asMap(v) {
		return k
	}

	return ""
}

func criticalExtensionField(v any, name string) ([]byte, error) {
	c1 := choiceValue(asMap(v)["criticalExtensions"])

	inner, ok := asMap(choiceValue(c1))[name].([]byte)
	if !ok {
		return nil, fmt.Errorf("rrc: %s absent from critical extensions", name)
	}

	return inner, nil
}

func containersFrom(items []any) []ratContainer {
	out := make([]ratContainer, 0, len(items))

	for _, it := range items {
		m := asMap(it)

		rat, _ := m["rat-Type"].(string)

		payload, ok := m["ue-CapabilityRAT-Container"].([]byte)
		if !ok {
			payload, _ = m["ueCapabilityRAT-Container"].([]byte)
		}

		out = append(out, ratContainer{ratType: rat, payload: payload})
	}

	return out
}

func parseContainerList(reg map[string]node, b []byte) ([]ratContainer, error) {
	v, err := decodeWith(reg, "UE-CapabilityRAT-ContainerList", b)
	if err != nil {
		return nil, fmt.Errorf("UE-CapabilityRAT-ContainerList: %w", err)
	}

	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("rrc: container list is not a sequence")
	}

	return containersFrom(items), nil
}

func parseUENRCapability(b []byte) (*NRCapability, error) {
	v, err := decodeWith(nrTypes, "UE-NR-Capability", b)
	if err != nil {
		return nil, fmt.Errorf("UE-NR-Capability: %w", err)
	}

	root := asMap(v)

	out := &NRCapability{}
	if s, ok := root["accessStratumRelease"].(string); ok {
		out.AccessStratumRelease = s
	}

	list, _ := asMap(root["rf-Parameters"])["supportedBandListNR"].([]any)

	for _, item := range list {
		m := asMap(item)

		band := NRBand{}
		if n, ok := m["bandNR"].(int64); ok {
			band.Band = n
		}

		if s, ok := m["ue-PowerClass"].(string); ok {
			band.PowerClass = s
		}

		_, band.PUSCH256QAM = m["pusch-256QAM"]
		band.ChannelBWDL = choiceKey(m["channelBWs-DL"])
		band.ChannelBWUL = choiceKey(m["channelBWs-UL"])

		out.Bands = append(out.Bands, band)
	}

	return out, nil
}

func parseUEEUTRACapability(b []byte) (*EUTRACapability, error) {
	v, err := decodeWith(eutraTypes, "UE-EUTRA-Capability", b)
	if err != nil {
		return nil, fmt.Errorf("UE-EUTRA-Capability: %w", err)
	}

	root := asMap(v)

	out := &EUTRACapability{}
	if s, ok := root["accessStratumRelease"].(string); ok {
		out.AccessStratumRelease = s
	}

	if n, ok := root["ue-Category"].(int64); ok {
		out.UECategory = n
	}

	list, _ := asMap(root["rf-Parameters"])["supportedBandListEUTRA"].([]any)

	for _, item := range list {
		m := asMap(item)

		band := EUTRABand{}
		if n, ok := m["bandEUTRA"].(int64); ok {
			band.Band = n
		}

		if hd, ok := m["halfDuplex"].(bool); ok {
			band.HalfDuplex = hd
		}

		out.Bands = append(out.Bands, band)
	}

	return out, nil
}

func capabilityFromContainers(containers []ratContainer) (*Capability, error) {
	out := &Capability{}

	for _, c := range containers {
		switch c.ratType {
		case "nr":
			nr, err := parseUENRCapability(c.payload)
			if err != nil {
				return nil, err
			}

			out.NR = nr
		case "eutra":
			eutra, err := parseUEEUTRACapability(c.payload)
			if err != nil {
				return nil, err
			}

			out.EUTRA = eutra
		}
	}

	if out.NR == nil && out.EUTRA == nil {
		return nil, fmt.Errorf("rrc: no NR or E-UTRA capability container")
	}

	return out, nil
}

func ParseNGAPUERadioCapability(b []byte) (*Capability, error) {
	v, err := decodeWith(nrTypes, "UERadioAccessCapabilityInformation", b)
	if err != nil {
		return nil, fmt.Errorf("UERadioAccessCapabilityInformation: %w", err)
	}

	inner, err := criticalExtensionField(v, "ue-RadioAccessCapabilityInfo")
	if err != nil {
		return nil, err
	}

	containers, err := parseContainerList(nrTypes, inner)
	if err != nil {
		return nil, err
	}

	return capabilityFromContainers(containers)
}

func ParseS1APUERadioCapability(b []byte) (*Capability, error) {
	v, err := decodeWith(eutraTypes, "UERadioAccessCapabilityInformation", b)
	if err != nil {
		return nil, fmt.Errorf("UERadioAccessCapabilityInformation: %w", err)
	}

	inner, err := criticalExtensionField(v, "ue-RadioAccessCapabilityInfo")
	if err != nil {
		return nil, err
	}

	msg, err := decodeWith(eutraTypes, "UECapabilityInformation", inner)
	if err != nil {
		return nil, fmt.Errorf("UECapabilityInformation: %w", err)
	}

	c1 := choiceValue(asMap(msg)["criticalExtensions"])

	list, ok := asMap(choiceValue(c1))["ue-CapabilityRAT-ContainerList"].([]any)
	if !ok {
		return nil, fmt.Errorf("rrc: ue-CapabilityRAT-ContainerList absent from UECapabilityInformation")
	}

	return capabilityFromContainers(containersFrom(list))
}
