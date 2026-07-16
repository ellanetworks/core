package main

import (
	"bytes"
	"fmt"
)

// isChoiceType reports whether a struct is a CHOICE: all fields have per: tags
// with choice: indices.
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

// choiceAlt holds a single CHOICE alternative.
type choiceAlt struct {
	fieldInfo
	rootIdx    int // index within root alternatives (-1 for extension additions)
	extIdx     int // index within extension additions (-1 for root)
	isAddition bool
}

// emitChoice emits MarshalPER and UnmarshalPER for a CHOICE type.
func (g *generator) emitChoice(name string, s *structType) error {
	recv := receiverName(name)

	// Split into root and extension additions.
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

	extensible := len(additions) > 0 || s.extSeq

	// nRoot = largest root choice index (§23.2: "n" = largest index in root).
	nRoot := int64(0)

	if len(roots) > 0 {
		for _, a := range roots {
			if int64(a.tag.ChoiceIdx) > nRoot {
				nRoot = int64(a.tag.ChoiceIdx)
			}
		}
	}

	g.emitHeader(name)
	g.emitChoiceMarshal(recv, name, roots, additions, extensible, nRoot)
	g.emitChoiceUnmarshal(recv, name, roots, additions, extensible, nRoot)

	return nil
}

func (g *generator) emitChoiceMarshal(recv, typeName string, roots, additions []choiceAlt, extensible bool, nRoot int64) {
	r := &g.buf
	fmt.Fprintf(r, "func (%s *%s) MarshalPER(w *per.Writer, enc per.Encoding) error {\n", recv, typeName)
	fmt.Fprintf(r, "\tswitch {\n")

	for _, a := range roots {
		expr := recv + "." + a.name
		fmt.Fprintf(r, "\tcase %s != nil:\n", expr)

		if extensible {
			fmt.Fprintf(r, "\t\tw.WriteBit(false)\n")
		}

		if nRoot > 0 {
			fmt.Fprintf(r, "\t\tper.EncodeConstrainedWholeNumber(w, enc, 0, %d, %d)\n", nRoot, a.tag.ChoiceIdx)
		}

		g.emitFieldMarshal(r, recv, a.fieldInfo, derefExpr(a, expr), 2)
	}

	for _, a := range additions {
		expr := recv + "." + a.name
		fmt.Fprintf(r, "\tcase %s != nil:\n", expr)
		fmt.Fprintf(r, "\t\tw.WriteBit(true)\n")
		fmt.Fprintf(r, "\t\tper.EncodeNormallySmall(w, enc, %d)\n", a.extIdx)
		fmt.Fprintf(r, "\t\treturn per.EncodeOpenType(w, enc, %s)\n", expr)
	}

	fmt.Fprintf(r, "\tdefault:\n")
	fmt.Fprintf(r, "\t\treturn per.ErrEmpty\n")
	fmt.Fprintf(r, "\t}\n")
	fmt.Fprintf(r, "\treturn nil\n}\n\n")
}

func (g *generator) emitChoiceUnmarshal(recv, typeName string, roots, additions []choiceAlt, extensible bool, nRoot int64) {
	r := &g.buf
	fmt.Fprintf(r, "func (%s *%s) UnmarshalPER(r *per.Reader, enc per.Encoding) error {\n", recv, typeName)

	if extensible {
		fmt.Fprintf(r, "\tisExt, err := r.ReadBit()\n")
		fmt.Fprintf(r, "\tif err != nil {\n\t\treturn err\n\t}\n")
		fmt.Fprintf(r, "\tif isExt {\n")
		g.emitChoiceExtDecode(r, recv, additions)
		fmt.Fprintf(r, "\t}\n")
	}

	// Root decode
	if nRoot > 0 {
		fmt.Fprintf(r, "\tidx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, %d)\n", nRoot)
		fmt.Fprintf(r, "\tif err != nil {\n\t\treturn err\n\t}\n")
	} else if len(roots) == 1 {
		fmt.Fprintf(r, "\tvar idx int64 = 0\n")
	} else {
		fmt.Fprintf(r, "\tvar idx int64 = 0\n")
	}

	fmt.Fprintf(r, "\tswitch idx {\n")

	for _, a := range roots {
		expr := recv + "." + a.name
		fmt.Fprintf(r, "\tcase %d:\n", a.tag.ChoiceIdx)
		fmt.Fprintf(r, "\t\tvar v %s\n", a.typeStr)
		g.emitFieldUnmarshal(r, "v", a.fieldInfo, 2)
		fmt.Fprintf(r, "\t\t%s = &v\n", expr)
	}

	fmt.Fprintf(r, "\tdefault:\n")
	fmt.Fprintf(r, "\t\treturn per.ErrOverflow\n")
	fmt.Fprintf(r, "\t}\n")
	fmt.Fprintf(r, "\treturn nil\n}\n\n")
}

func (g *generator) emitChoiceExtDecode(r *bytes.Buffer, recv string, additions []choiceAlt) {
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
		fmt.Fprintf(r, "\t\t\tif err := per.DecodeOpenType(r, enc, &v); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n")
		fmt.Fprintf(r, "\t\t\t%s = &v\n", expr)
	}

	fmt.Fprintf(r, "\t\tdefault:\n")
	fmt.Fprintf(r, "\t\t\treturn per.SkipOpenType(r, enc)\n")
	fmt.Fprintf(r, "\t\t}\n")
	fmt.Fprintf(r, "\t\treturn nil\n")
}

// derefExpr returns the Go expression for accessing a CHOICE alternative's
// value (dereferenced if it's a pointer field, which CHOICE alternatives are).
func derefExpr(a choiceAlt, expr string) string {
	if a.isOptional {
		return "(*" + expr + ")"
	}

	return expr
}
