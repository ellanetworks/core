// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pergen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/types"
)

const (
	apImportPath   = "github.com/ellanetworks/core/per/ap/apimpl"
	apPublicPath   = "github.com/ellanetworks/core/per/ap"
	apInternalPath = "github.com/ellanetworks/core/per/ap/internal/impl"
)

type enumInfo struct {
	varName    string
	extensible bool
}

func (g *generator) collectEnums(names []string) error {
	g.enums = make(map[string]enumInfo)

	for _, name := range names {
		v, ok := g.pkg.Scope().Lookup(name).(*types.Var)
		if !ok {
			continue
		}

		n, ok := types.Unalias(v.Type()).(*types.Named)
		if !ok {
			continue
		}

		obj := n.Origin().Obj()
		if obj.Pkg() == nil || obj.Pkg().Path() != perImportPath || obj.Name() != "Enum" {
			continue
		}

		arg, ok := types.Unalias(n.TypeArgs().At(0)).(*types.Named)
		if !ok || arg.Obj().Pkg() != g.pkg {
			return fmt.Errorf("%s: per.Enum must describe a named integer type of this package", name)
		}

		typeName := arg.Obj().Name()
		if prev, dup := g.enums[typeName]; dup {
			return fmt.Errorf("%s: %s is already described by %s", name, typeName, prev.varName)
		}

		ext, err := g.enumExtensible(name)
		if err != nil {
			return err
		}

		g.enums[typeName] = enumInfo{varName: name, extensible: ext}
		g.genTypes[typeName] = true
	}

	return nil
}

func (g *generator) enumExtensible(varName string) (bool, error) {
	for _, f := range g.files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}

				for i, id := range vs.Names {
					if id.Name != varName {
						continue
					}

					if i >= len(vs.Values) {
						return false, fmt.Errorf("%s: per.Enum needs a composite literal", varName)
					}

					lit, ok := vs.Values[i].(*ast.CompositeLit)
					if !ok {
						return false, fmt.Errorf("%s: per.Enum needs a composite literal", varName)
					}

					return compositeExtensible(lit), nil
				}
			}
		}
	}

	return false, fmt.Errorf("%s: declaration not found", varName)
}

func compositeExtensible(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "Extensible":
			if v, ok := kv.Value.(*ast.Ident); ok && v.Name == "true" {
				return true
			}
		case "Extension":
			if l, ok := kv.Value.(*ast.CompositeLit); ok && len(l.Elts) > 0 {
				return true
			}
		}
	}

	return false
}

func (g *generator) isEnum(n *types.Named) bool {
	return n.Obj().Pkg() == g.pkg && g.enums[n.Obj().Name()].varName != ""
}

func (g *generator) emitEnum(name string, e enumInfo) {
	r := &g.buf
	recv := g.receiver(name)

	fmt.Fprintf(r, "func (%s %s) MarshalPER(w *per.Writer, enc per.Encoding) error {\n", recv, name)
	fmt.Fprintf(r, "\treturn %s.Encode(w, enc, %s)\n}\n\n", e.varName, recv)
	fmt.Fprintf(r, "func (%s *%s) UnmarshalPER(r *per.Reader, enc per.Encoding) error {\n", recv, name)
	fmt.Fprintf(r, "\tv, err := %s.Decode(r, enc)\n\tif err != nil {\n\t\treturn err\n\t}\n", e.varName)
	fmt.Fprintf(r, "\t*%s = v\n\treturn nil\n}\n\n", recv)
	fmt.Fprintf(r, "func (%s %s) Name() string {\n\treturn %s.Name(%s)\n}\n\n", recv, name, e.varName, recv)

	if g.apMode() && e.extensible {
		g.usesAP = true

		fmt.Fprintf(r, "func (%s %s) Diagnose(*apimpl.Diagnosis[%s]) bool {\n\treturn !%s.Known(%s)\n}\n\n", recv, name, g.cfg.IEID, e.varName, recv)
	}
}

func (g *generator) sliceTypes(names []string) []string {
	var out []string

	for _, name := range names {
		nt, ok := g.pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok || nt.IsAlias() {
			continue
		}

		named, ok := nt.Type().(*types.Named)
		if !ok || hasDeclaredMethod(named, "Diagnose") {
			continue
		}

		if _, ok := named.Underlying().(*types.Slice); ok {
			out = append(out, name)
		}
	}

	return out
}

func (g *generator) emitListDiagnose(name string) {
	g.usesAP = true

	r := &g.buf
	recv := g.receiver(name)

	fmt.Fprintf(r, "func (%s %s) Diagnose(d *apimpl.Diagnosis[%s]) bool {\n", recv, name, g.cfg.IEID)
	fmt.Fprintf(r, "\tnotComprehended := false\n")
	fmt.Fprintf(r, "\tfor i := range %s {\n\t\tif %s[i].Diagnose(d) {\n\t\t\tnotComprehended = true\n\t\t}\n\t}\n", recv, recv)
	fmt.Fprintf(r, "\treturn notComprehended\n}\n\n")
}

func (g *generator) diagnosable(structs map[string]*structType, lists []string) map[string]bool {
	diag := make(map[string]bool)

	for name, e := range g.enums {
		if e.extensible {
			diag[name] = true
		}
	}

	for name, st := range structs {
		if st.extAlt != "" || len(st.extIEs) > 0 {
			diag[name] = true
		}

		for _, fi := range st.fields {
			if fi.genTable || fi.rawSingle || (fi.tag.ExtIE && fi.tag.ChoiceIdx >= 0) {
				diag[name] = true
			}
		}
	}

	for changed := true; changed; {
		changed = false

		for _, name := range lists {
			sl := g.pkg.Scope().Lookup(name).Type().Underlying().(*types.Slice)
			if !diag[name] && g.typeDiagnosable(diag, sl.Elem()) {
				diag[name] = true
				changed = true
			}
		}

		for name, st := range structs {
			if diag[name] {
				continue
			}

			for _, fi := range st.fields {
				if g.fieldDiagnosable(diag, fi) != diagNone {
					diag[name] = true
					changed = true

					break
				}
			}
		}
	}

	return diag
}

type diagShape uint8

const (
	diagNone diagShape = iota
	diagValue
	diagElems
)

func (g *generator) fieldDiagnosable(diag map[string]bool, fi fieldInfo) diagShape {
	if fi.genTable || fi.rawSingle || fi.extTable != "" {
		return diagNone
	}

	if fi.kind == kindSequenceOf {
		if g.typeDiagnosable(diag, fi.elemType) {
			return diagElems
		}

		return diagNone
	}

	if fi.kind != kindDelegate || fi.delegateNamed == nil {
		return diagNone
	}

	if g.typeDiagnosable(diag, fi.delegateNamed) {
		return diagValue
	}

	if sl, ok := fi.delegateNamed.Underlying().(*types.Slice); ok && g.typeDiagnosable(diag, sl.Elem()) {
		return diagElems
	}

	return diagNone
}

func (g *generator) typeDiagnosable(diag map[string]bool, t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}

	if n.Obj().Pkg() == g.pkg && diag[n.Obj().Name()] {
		return true
	}

	return hasDeclaredMethod(n, "Diagnose")
}

func (g *generator) emitDiagnose(name string, st *structType) {
	g.usesAP = true

	r := &g.buf
	recv := g.receiver(name)

	fmt.Fprintf(r, "func (%s *%s) Diagnose(d *apimpl.Diagnosis[%s]) bool {\n", recv, name, g.cfg.IEID)

	if isChoiceType(st) {
		g.emitChoiceDiagnose(r, recv, st)

		return
	}

	fmt.Fprintf(r, "\tnotComprehended := false\n")

	for _, fi := range st.fields {
		expr := recv + "." + fi.name

		if fi.genTable {
			fmt.Fprintf(r, "\tapimpl.DiagnoseExtensions(d, %s, %s, &%s)\n", fi.extTable, recv, expr)

			continue
		}

		switch g.fieldDiagnosable(g.diagTypes, fi) {
		case diagValue:
			if fi.isOptional && !fi.noDeref {
				fmt.Fprintf(r, "\tif %s != nil && %s.Diagnose(d) {\n\t\tnotComprehended = true\n\t}\n", expr, expr)
			} else {
				fmt.Fprintf(r, "\tif %s.Diagnose(d) {\n\t\tnotComprehended = true\n\t}\n", expr)
			}
		case diagElems:
			slice := expr
			if fi.isOptional && !fi.noDeref {
				fmt.Fprintf(r, "\tif %s != nil {\n", expr)

				slice = "(*" + expr + ")"
			}

			fmt.Fprintf(r, "\tfor i := range %s {\n\t\tif %s[i].Diagnose(d) {\n\t\t\tnotComprehended = true\n\t\t}\n\t}\n", slice, slice)

			if fi.isOptional && !fi.noDeref {
				fmt.Fprintf(r, "\t}\n")
			}
		case diagNone:
		}
	}

	fmt.Fprintf(r, "\treturn notComprehended\n}\n\n")
}

func (g *generator) emitChoiceDiagnose(r *bytes.Buffer, recv string, st *structType) {
	raw := ""

	for _, fi := range st.fields {
		if fi.rawSingle {
			raw = recv + "." + fi.name
		}
	}

	fmt.Fprintf(r, "\tswitch {\n")

	for _, fi := range st.fields {
		expr := recv + "." + fi.name

		fmt.Fprintf(r, "\tcase %s != nil:\n", expr)

		switch {
		case fi.rawSingle:
			fmt.Fprintf(r, "\t\treturn apimpl.DiagnoseUnknownChoiceExtension(d, %s)\n", expr)
		case fi.tag.ExtIE:
			fmt.Fprintf(r, "\t\treturn apimpl.DiagnoseChoiceExtension(d, %s, %s, &%s, &%s)\n", fi.tag.ExtIEID, g.choiceCritExpr(recv, st, fi), expr, raw)
		case g.fieldDiagnosable(g.diagTypes, fi) == diagValue:
			fmt.Fprintf(r, "\t\treturn %s.Diagnose(d)\n", expr)
		}
	}

	if st.extAlt != "" {
		fmt.Fprintf(r, "\tcase %s.%s != nil:\n\t\treturn true\n", recv, st.extAlt)
	}

	fmt.Fprintf(r, "\t}\n\treturn false\n}\n\n")
}

func (g *generator) choiceCritExpr(recv string, st *structType, fi fieldInfo) string {
	spec := critConst(fi.tag.Criticality)
	if st.choiceCrit == "" {
		return spec
	}

	return recv + "." + st.choiceCrit + ".Of(" + spec + ")"
}

func critConst(c string) string {
	switch c {
	case "reject":
		return "apimpl.CriticalityReject"
	case "notify":
		return "apimpl.CriticalityNotify"
	default:
		return "apimpl.CriticalityIgnore"
	}
}

func presenceConst(p string) string {
	switch p {
	case "mandatory":
		return "apimpl.PresenceMandatory"
	case "conditional":
		return "apimpl.PresenceConditional"
	default:
		return "apimpl.PresenceOptional"
	}
}

func (g *generator) emitExtTable(name string, st *structType) {
	g.usesAP = true

	r := &g.buf
	table := extTableName(name)

	if len(st.extIEs) == 0 {
		fmt.Fprintf(r, "var %s []apimpl.IESpec[%s, %s]\n\n", table, g.cfg.IEID, name)

		return
	}

	fmt.Fprintf(r, "var %s = []apimpl.IESpec[%s, %s]{\n", table, g.cfg.IEID, name)

	for _, fi := range st.extIEs {
		expr := "x." + fi.name

		fmt.Fprintf(r, "\t{\n\t\tID: %s,\n\t\tPresence: %s,\n\t\tCriticality: %s,\n", fi.tag.ExtIEID, presenceConst(fi.tag.Presence), critConst(fi.tag.Criticality))

		if fi.tag.Condition != "" {
			fmt.Fprintf(r, "\t\tCondition: %s,\n", fi.tag.Condition)
		}

		fmt.Fprintf(r, "\t\tDecode: func(x *%s, r *per.Reader, enc per.Encoding) error {\n", name)
		fmt.Fprintf(r, "\t\t\tvar v %s\n", fi.typeStr)
		g.emitFieldUnmarshal(r, "v", fi, 2)

		val, absent := "(*"+expr+")", expr+" == nil"
		if fi.noDeref {
			fmt.Fprintf(r, "\t\t\t%s = v\n\t\t\treturn nil\n\t\t},\n", expr)

			val, absent = expr, "len("+expr+") == 0"
		} else {
			fmt.Fprintf(r, "\t\t\t%s = &v\n\t\t\treturn nil\n\t\t},\n", expr)
		}

		fmt.Fprintf(r, "\t\tEncode: func(x *%s) (per.Marshaler, bool) {\n", name)
		fmt.Fprintf(r, "\t\t\tif %s {\n\t\t\t\treturn nil, false\n\t\t\t}\n", absent)

		if fi.kind == kindDelegate {
			fmt.Fprintf(r, "\t\t\treturn %s, true\n", expr)
		} else {
			fmt.Fprintf(r, "\t\t\treturn per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {\n")
			g.emitFieldMarshal(r, "x", fi, val, 3)
			fmt.Fprintf(r, "\t\t\t\treturn nil\n\t\t\t}), true\n")
		}

		fmt.Fprintf(r, "\t\t},\n\t\tClear: func(x *%s) {\n\t\t\t%s = nil\n", name, expr)
		fmt.Fprintf(r, "\t\t},\n\t},\n")
	}

	fmt.Fprintf(r, "}\n\n")
}

func hasGenTable(st *structType) bool {
	for _, fi := range st.fields {
		if fi.genTable {
			return true
		}
	}

	return false
}

func (g *generator) receiver(typeName string) string {
	recv := receiverName(typeName)
	if g.apMode() && recv == typeName {
		return "x"
	}

	return recv
}

func (g *generator) openTypeDecoder() string {
	if g.apMode() {
		return "per.DecodeCompleteOpenType"
	}

	return "per.DecodeOpenType"
}
