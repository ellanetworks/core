// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

// TS 23.501 §5.8.2.7
func TestDscpForQos(t *testing.T) {
	tests := []struct {
		name   string
		fiveQI uint8
		arp    models.Arp
		want   uint8
	}{
		{"voice", 1, models.Arp{PriorityLevel: 15}, 46},
		{"mc push to talk voice", 65, models.Arp{PriorityLevel: 9}, 46},
		{"non-mc push to talk voice", 66, models.Arp{PriorityLevel: 9}, 46},
		{"delay critical gbr discrete automation", 82, models.Arp{PriorityLevel: 1}, 46},
		{"delay critical gbr intelligent transport", 84, models.Arp{PriorityLevel: 1}, 46},
		{"delay critical gbr electricity distribution", 85, models.Arp{PriorityLevel: 1}, 46},
		{"delay critical gbr v2x collision avoidance", 86, models.Arp{PriorityLevel: 15}, 46},
		{"delay critical gbr motion tracking", 87, models.Arp{PriorityLevel: 15}, 46},
		{"delay critical gbr split rendering", 90, models.Arp{PriorityLevel: 15}, 46},
		{"conversational video", 2, models.Arp{PriorityLevel: 15}, 34},
		{"mission critical video", 67, models.Arp{PriorityLevel: 1}, 34},
		{"live uplink streaming", 71, models.Arp{PriorityLevel: 10}, 34},
		{"v2x messages gbr", 75, models.Arp{PriorityLevel: 10}, 34},
		{"live uplink streaming high packet loss", 76, models.Arp{PriorityLevel: 10}, 34},
		{"real time gaming", 3, models.Arp{PriorityLevel: 15}, 36},
		{"buffered gbr video", 4, models.Arp{PriorityLevel: 15}, 38},
		{"ims signalling", 5, models.Arp{PriorityLevel: 15}, 40},
		{"mission critical delay sensitive signalling", 69, models.Arp{PriorityLevel: 1}, 48},
		{"best effort without priority", 9, models.Arp{}, 0},
		{"best effort lowest priority", 9, models.Arp{PriorityLevel: 15}, 0},
		{"best effort high priority", 9, models.Arp{PriorityLevel: 1}, 18},
		{"best effort priority five", 9, models.Arp{PriorityLevel: 5}, 18},
		{"best effort priority six", 9, models.Arp{PriorityLevel: 6}, 10},
		{"best effort priority ten", 9, models.Arp{PriorityLevel: 10}, 10},
		{"best effort priority eleven", 9, models.Arp{PriorityLevel: 11}, 0},
		{"buffered streaming high priority", 6, models.Arp{PriorityLevel: 2}, 18},
		{"interactive gaming medium priority", 7, models.Arp{PriorityLevel: 7}, 10},
		{"mc data without priority", 70, models.Arp{}, 0},
		{"v2x messages without priority", 79, models.Arp{}, 0},
		{"low latency embb without priority", 80, models.Arp{}, 0},
		{"operator specific non gbr high priority", 128, models.Arp{PriorityLevel: 1}, 18},
		{"zero five qi", 0, models.Arp{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dscpForQos(tt.fiveQI, tt.arp); got != tt.want {
				t.Errorf("dscpForQos(%d, %+v) = %d, want %d", tt.fiveQI, tt.arp, got, tt.want)
			}
		})
	}
}

// TS 29.244 §8.2.12
func TestTransportLevelMarking(t *testing.T) {
	tests := []struct {
		name   string
		fiveQI uint8
		arp    models.Arp
		want   uint16
	}{
		{"voice encodes the tos byte and mask", 1, models.Arp{PriorityLevel: 15}, 0xB8FC},
		{"ims signalling encodes the tos byte and mask", 5, models.Arp{PriorityLevel: 15}, 0xA0FC},
		{"best effort high priority encodes the tos byte and mask", 9, models.Arp{PriorityLevel: 1}, 0x48FC},
		{"best effort stays unmarked", 9, models.Arp{PriorityLevel: 15}, 0},
		{"zero five qi stays unmarked", 0, models.Arp{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transportLevelMarking(tt.fiveQI, tt.arp); got != tt.want {
				t.Errorf("transportLevelMarking(%d, %+v) = %#04x, want %#04x", tt.fiveQI, tt.arp, got, tt.want)
			}
		})
	}
}

func TestArpValue(t *testing.T) {
	if got := arpValue(nil); got != (models.Arp{}) {
		t.Errorf("arpValue(nil) = %+v, want the zero ARP", got)
	}

	arp := models.Arp{PriorityLevel: 7}
	if got := arpValue(&arp); got != arp {
		t.Errorf("arpValue(%+v) = %+v, want %+v", arp, got, arp)
	}
}
