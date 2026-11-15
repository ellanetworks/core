// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pergen

import (
	"bytes"
	"fmt"
	"strings"
)

// isChoiceType requires every field to carry a choice: index, so one untagged
// field makes the type a SEQUENCE.
func isChoiceType(s *structType) bool {
	if len(s.fields) == 0 {
		return false
	}

	for i := range s.fields {
		if !s.fields[i].has || s.fields[i].tag.ChoiceIdx < 0 {
			return false
		}
	}

	return true
}

type choiceAlt struct {
	fieldInfo
	rootIdx    int // -1 for extension additions
	extIdx     int // -1 for root alternatives
	isAddition bool
}

func (g *generator) emitChoice(name string, s *structType) error {
	recv := g.receiver(name)

	var roots, additions []choiceAlt

	for i := range s.fields {
		fi := s.fields[i]

		alt := choiceAlt{fieldInfo: fi, rootIdx: -1, extIdx: -1}
		if fi.tag.Ext {
			alt.isAddition = true
			alt.extIdx = len(additions)
			additions = append(additions, alt)
		} else {
			alt.rootIdx = len(roots)
			roots = append(roots, alt)
		}
	}

	extensible := len(additions) > 0 || s.extSeq || s.extAlt != ""

	// X.691 §23.2: the index range is 0..n, n being the largest root index.
	nRoot := int64(0)

	if len(roots) > 0 {
		for _, a := range roots {
			if int64(a.tag.ChoiceIdx) > nRoot {
				nRoot = int64(a.tag.ChoiceIdx)
			}
		}
	}

	raw := false

	for _, a := range roots {
		if a.tag.ExtIE && !a.rawSingle && a.kind != kindDelegate {
			return fmt.Errorf("field %s: a choice-Extensions alternative must be a type with its own codec", a.name)
		}

		raw = raw || a.rawSingle
	}

	for _, a := range roots {
		if a.tag.ExtIE && !raw {
			return fmt.Errorf("field %s: choice-Extensions needs a bare extie field of type *apimpl.SingleContainer for unknown alternatives", a.name)
		}
	}

	g.cur = s
	g.emitChoiceMarshal(recv, name, roots, additions, extensible, nRoot)
	g.emitChoiceUnmarshal(recv, name, roots, additions, extensible, nRoot)

	return nil
}

func (g *generator) emitChoiceMarshal(recv, typeName string, roots, additions []choiceAlt, extensible bool, nRoot int64) {
	r := &g.buf
	fmt.Fprintf(r, "func (%s *%s) MarshalPER(w *per.Writer, enc per.Encoding) error {\n", recv, typeName)

	if g.apMode() {
		var set []string

		for _, a := range append(append([]choiceAlt{}, roots...), additions...) {
			set = append(set, recv+"."+a.name+" != nil")
		}

		if g.cur.extAlt != "" {
			set = append(set, recv+"."+g.cur.extAlt+" != nil")
		}

		fmt.Fprintf(r, "\tset := 0\n\tfor _, ok := range [...]bool{%s} {\n\t\tif ok {\n\t\t\tset++\n\t\t}\n\t}\n", strings.Join(set, ", "))
		fmt.Fprintf(r, "\tif set > 1 {\n\t\treturn per.ErrChoice\n\t}\n")
	}

	fmt.Fprintf(r, "\tswitch {\n")

	for _, a := range roots {
		expr := recv + "." + a.name
		fmt.Fprintf(r, "\tcase %s != nil:\n", expr)

		if extensible {
			fmt.Fprintf(r, "\t\tw.WriteBit(false)\n")
		}

		if nRoot > 0 {
			fmt.Fprintf(r, "\t\tif err := per.EncodeConstrainedWholeNumber(w, enc, 0, %d, %d); err != nil {\n", nRoot, a.tag.ChoiceIdx)
			fmt.Fprintf(r, "\t\t\treturn err\n")
			fmt.Fprintf(r, "\t\t}\n")
		}

		switch {
		case a.rawSingle:
			fmt.Fprintf(r, "\t\treturn apimpl.EncodeField(%s, w, enc, %s.Field())\n", g.cfg.Profile, expr)
		case a.tag.ExtIE:
			fmt.Fprintf(r, "\t\treturn apimpl.EncodeField(%s, w, enc, apimpl.FieldValue[%s]{ID: %s, Criticality: %s, Value: %s})\n",
				g.cfg.Profile, g.cfg.IEID, a.tag.ExtIEID, g.choiceCritExpr(recv, g.cur, a.fieldInfo), expr)
		default:
			g.emitFieldMarshal(r, recv, a.fieldInfo, derefExpr(a, expr), 2)
		}
	}

	for _, a := range additions {
		expr := recv + "." + a.name
		fmt.Fprintf(r, "\tcase %s != nil:\n", expr)
		fmt.Fprintf(r, "\t\tw.WriteBit(true)\n")
		fmt.Fprintf(r, "\t\tif err := per.EncodeNormallySmall(w, enc, %d); err != nil {\n", a.extIdx)
		fmt.Fprintf(r, "\t\t\treturn err\n")
		fmt.Fprintf(r, "\t\t}\n")
		fmt.Fprintf(r, "\t\treturn per.EncodeOpenType(w, enc, %s)\n", expr)
	}

	if g.cur.extAlt != "" {
		fmt.Fprintf(r, "\tcase %s.%s != nil:\n", recv, g.cur.extAlt)
		fmt.Fprintf(r, "\t\tw.WriteBit(true)\n")
		fmt.Fprintf(r, "\t\treturn %s.%s.MarshalPER(w, enc)\n", recv, g.cur.extAlt)
	}

	fmt.Fprintf(r, "\tdefault:\n")
	fmt.Fprintf(r, "\t\treturn per.ErrEmpty\n")
	fmt.Fprintf(r, "\t}\n")

	if choiceFallsThrough(roots) {
		fmt.Fprintf(r, "\treturn nil\n")
	}

	fmt.Fprintf(r, "}\n\n")
}

func choiceFallsThrough(roots []choiceAlt) bool {
	for _, a := range roots {
		if !a.rawSingle && !a.tag.ExtIE {
			return true
		}
	}

	return false
}

func (g *generator) emitChoiceUnmarshal(recv, typeName string, roots, additions []choiceAlt, extensible bool, nRoot int64) {
	r := &g.buf
	fmt.Fprintf(r, "func (%s *%s) UnmarshalPER(r *per.Reader, enc per.Encoding) error {\n", recv, typeName)

	if g.apMode() {
		fmt.Fprintf(r, "\t*%s = %s{}\n", recv, typeName)
	}

	if extensible {
		fmt.Fprintf(r, "\tisExt, err := r.ReadBit()\n")
		fmt.Fprintf(r, "\tif err != nil {\n\t\treturn err\n\t}\n")
		fmt.Fprintf(r, "\tif isExt {\n")
		g.emitChoiceExtDecode(r, recv, additions)
		fmt.Fprintf(r, "\t}\n")
	}

	if nRoot > 0 {
		fmt.Fprintf(r, "\tidx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, %d)\n", nRoot)
		fmt.Fprintf(r, "\tif err != nil {\n\t\treturn err\n\t}\n")
	} else if len(roots) == 1 {
		fmt.Fprintf(r, "\tvar idx int64 = 0\n")
	} else {
		fmt.Fprintf(r, "\tvar idx int64 = 0\n")
	}

	fmt.Fprintf(r, "\tswitch idx {\n")

	var singles []choiceAlt

	for _, a := range roots {
		if a.tag.ExtIE {
			singles = append(singles, a)

			continue
		}

		expr := recv + "." + a.name
		fmt.Fprintf(r, "\tcase %d:\n", a.tag.ChoiceIdx)
		fmt.Fprintf(r, "\t\tvar v %s\n", a.typeStr)
		g.emitFieldUnmarshal(r, "v", a.fieldInfo, 2)
		fmt.Fprintf(r, "\t\t%s = &v\n", expr)
	}

	if len(singles) > 0 {
		g.emitChoiceSingleDecode(r, recv, singles)
	}

	fmt.Fprintf(r, "\tdefault:\n")
	fmt.Fprintf(r, "\t\treturn per.ErrOverflow\n")
	fmt.Fprintf(r, "\t}\n")
	fmt.Fprintf(r, "\treturn nil\n}\n\n")
}

func (g *generator) emitChoiceSingleDecode(r *bytes.Buffer, recv string, singles []choiceAlt) {
	fmt.Fprintf(r, "\tcase %d:\n", singles[0].tag.ChoiceIdx)
	fmt.Fprintf(r, "\t\tf, err := apimpl.DecodeSingleContainerValue[%s](%s, r, enc)\n", g.cfg.IEID, g.cfg.Profile)
	fmt.Fprintf(r, "\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")
	fmt.Fprintf(r, "\t\tswitch f.ID {\n")

	var raw string

	for _, a := range singles {
		if a.tag.ChoiceIdx != singles[0].tag.ChoiceIdx {
			continue
		}

		expr := recv + "." + a.name
		if a.rawSingle {
			raw = expr

			continue
		}

		fmt.Fprintf(r, "\t\tcase %s:\n", a.tag.ExtIEID)
		fmt.Fprintf(r, "\t\t\tvar v %s\n", a.typeStr)
		fmt.Fprintf(r, "\t\t\tif err := apimpl.DecodeValue([]byte(f.Value), &v); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n")
		fmt.Fprintf(r, "\t\t\t%s = &v\n", expr)

		if g.cur.choiceCrit != "" {
			fmt.Fprintf(r, "\t\t\t%s.%s = apimpl.Wire(f.Criticality, %s)\n", recv, g.cur.choiceCrit, critConst(a.tag.Criticality))
		}
	}

	fmt.Fprintf(r, "\t\tdefault:\n")

	fmt.Fprintf(r, "\t\t\t%s = &f\n", raw)

	fmt.Fprintf(r, "\t\t}\n")
}

func (g *generator) emitChoiceExtDecode(r *bytes.Buffer, recv string, additions []choiceAlt) {
	if g.cur.extAlt != "" {
		fmt.Fprintf(r, "\t\textIdx, err := per.DecodeNormallySmall(r, enc)\n")
		fmt.Fprintf(r, "\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")

		if len(additions) == 0 {
			fmt.Fprintf(r, "\t\t%s.%s, err = per.DecodeExtensionAlternative(r, enc, extIdx)\n", recv, g.cur.extAlt)
			fmt.Fprintf(r, "\t\treturn err\n")

			return
		}

		fmt.Fprintf(r, "\t\tswitch extIdx {\n")

		for _, a := range additions {
			expr := recv + "." + a.name
			fmt.Fprintf(r, "\t\tcase %d:\n", a.extIdx)
			fmt.Fprintf(r, "\t\t\tvar v %s\n", a.typeStr)
			fmt.Fprintf(r, "\t\t\tif err := %s(r, enc, &v); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n", g.openTypeDecoder())
			fmt.Fprintf(r, "\t\t\t%s = &v\n", expr)
		}

		fmt.Fprintf(r, "\t\tdefault:\n")
		fmt.Fprintf(r, "\t\t\t%s.%s, err = per.DecodeExtensionAlternative(r, enc, extIdx)\n", recv, g.cur.extAlt)
		fmt.Fprintf(r, "\t\t\treturn err\n")
		fmt.Fprintf(r, "\t\t}\n")
		fmt.Fprintf(r, "\t\treturn nil\n")

		return
	}

	if len(additions) == 0 {
		fmt.Fprintf(r, "\t\tif _, err := per.DecodeNormallySmall(r, enc); err != nil {\n")
		fmt.Fprintf(r, "\t\t\treturn err\n")
		fmt.Fprintf(r, "\t\t}\n")
		fmt.Fprintf(r, "\t\treturn per.SkipOpenType(r, enc)\n")

		return
	}

	fmt.Fprintf(r, "\t\textIdx, err := per.DecodeNormallySmall(r, enc)\n")
	fmt.Fprintf(r, "\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")
	fmt.Fprintf(r, "\t\tswitch extIdx {\n")

	for _, a := range additions {
		expr := recv + "." + a.name
		fmt.Fprintf(r, "\t\tcase %d:\n", a.extIdx)
		fmt.Fprintf(r, "\t\t\tvar v %s\n", a.typeStr)
		fmt.Fprintf(r, "\t\t\tif err := %s(r, enc, &v); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n", g.openTypeDecoder())
		fmt.Fprintf(r, "\t\t\t%s = &v\n", expr)
	}

	fmt.Fprintf(r, "\t\tdefault:\n")
	fmt.Fprintf(r, "\t\t\treturn per.SkipOpenType(r, enc)\n")
	fmt.Fprintf(r, "\t\t}\n")
	fmt.Fprintf(r, "\t\treturn nil\n")
}

func derefExpr(a choiceAlt, expr string) string {
	if a.isOptional {
		return "(*" + expr + ")"
	}

	return expr
}
