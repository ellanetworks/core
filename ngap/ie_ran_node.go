// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// GlobalRANNodeID ::= CHOICE { globalGNB-ID, globalNgENB-ID, globalN3IWF-ID,
// choice-Extensions } root alternatives (TS 38.413 §9.3.1.5). The CHOICE is
// not extensible, so choice-Extensions is a plain alternative index.
const (
	globalGNBID = iota
	globalNgENBID
	globalN3IWFID
	globalRANNodeIDChoiceExtensions

	globalRANNodeIDAlternatives = 4
)

// RANNodeIDKind selects a GlobalRANNodeID alternative together with the
// alternative of the node-id CHOICE nested inside it. The two levels are
// flattened because every combination names one kind of RAN node, and a
// caller that has to switch on the outer CHOICE always switches on the inner
// one too.
type RANNodeIDKind uint8

const (
	// RANNodeIDGNB is globalGNB-ID / gNB-ID: BIT STRING (SIZE(22..32)).
	RANNodeIDGNB RANNodeIDKind = iota
	// RANNodeIDMacroNgENB is globalNgENB-ID / macroNgENB-ID: SIZE(20).
	RANNodeIDMacroNgENB
	// RANNodeIDShortMacroNgENB is globalNgENB-ID / shortMacroNgENB-ID: SIZE(18).
	RANNodeIDShortMacroNgENB
	// RANNodeIDLongMacroNgENB is globalNgENB-ID / longMacroNgENB-ID: SIZE(21).
	RANNodeIDLongMacroNgENB
	// RANNodeIDN3IWF is globalN3IWF-ID / n3IWF-ID: BIT STRING (SIZE(16)).
	RANNodeIDN3IWF
)

// ranNodeIDShape is how one kind sits on the wire: which alternative of each
// CHOICE level selects it, and the SIZE constraint of its BIT STRING.
type ranNodeIDShape struct {
	outer  int
	inner  int
	lb, ub int
}

var ranNodeIDShapes = map[RANNodeIDKind]ranNodeIDShape{
	RANNodeIDGNB:             {globalGNBID, 0, 22, 32},
	RANNodeIDMacroNgENB:      {globalNgENBID, 0, 20, 20},
	RANNodeIDShortMacroNgENB: {globalNgENBID, 1, 18, 18},
	RANNodeIDLongMacroNgENB:  {globalNgENBID, 2, 21, 21},
	RANNodeIDN3IWF:           {globalN3IWFID, 0, 16, 16},
}

// nodeIDAlternatives is the number of alternatives in each Global*-ID's nested
// node-id CHOICE, including its choice-Extensions. None is extensible.
var nodeIDAlternatives = map[int]int{
	globalGNBID:   2, // gNB-ID, choice-Extensions
	globalNgENBID: 4, // macroNgENB-ID, shortMacroNgENB-ID, longMacroNgENB-ID, choice-Extensions
	globalN3IWFID: 2, // n3IWF-ID, choice-Extensions
}

// nodeIDChoiceName names the nested CHOICE for error messages.
var nodeIDChoiceName = map[int]string{
	globalGNBID:   "GNB-ID",
	globalNgENBID: "NgENB-ID",
	globalN3IWFID: "N3IWF-ID",
}

// kindForShape inverts ranNodeIDShapes for decoding.
var kindForShape = func() map[[2]int]RANNodeIDKind {
	m := make(map[[2]int]RANNodeIDKind, len(ranNodeIDShapes))
	for kind, s := range ranNodeIDShapes {
		m[[2]int{s.outer, s.inner}] = kind
	}

	return m
}()

// GlobalRANNodeID identifies a RAN node: the PLMN it belongs to and its node
// identifier, whose meaning is set by Kind (TS 38.413 §9.3.1.5).
//
// Each of globalGNB-ID, globalNgENB-ID and globalN3IWF-ID is an extensible
// SEQUENCE of a PLMN identity and a node-id CHOICE, so the three flatten into
// one struct without losing anything the wire carries.
type GlobalRANNodeID struct {
	Kind         RANNodeIDKind
	PLMNIdentity PLMNIdentity

	// Value is the node identifier, right-aligned in Bits bits.
	Value uint32

	// Bits is the identifier's BIT STRING length. Only RANNodeIDGNB has a
	// range to choose from (22..32); every other kind has exactly one legal
	// length, and encoding rejects anything else rather than guessing.
	Bits int
}
