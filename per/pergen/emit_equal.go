// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pergen

import (
	"fmt"
	"go/types"
	"slices"
	"sort"
	"strings"
)

func (g *generator) equalTypes(structs map[string]*structType, names []string) []string {
	eq := make(map[string]bool)

	for _, name := range names {
		named, ok := g.localNamed(name)
		if !ok || g.hasEqual(named) {
			continue
		}

		switch u := named.Underlying().(type) {
		case *types.Struct:
			if structs[name] != nil || g.isMessage(named) || hasDeclaredMethod(named, "MarshalPER") {
				eq[name] = true
			}
		case *types.Slice:
			if n, ok := types.Unalias(u.Elem()).(*types.Named); ok && (g.implementsMarshalPER(n) || hasDeclaredMethod(named, "MarshalPER")) {
				eq[name] = true
			}
		}
	}

	g.eqTypes = eq

	for changed := true; changed; {
		changed = false

		for _, name := range sortedKeys(eq) {
			named, _ := g.localNamed(name)

			for _, t := range componentTypes(named.Underlying()) {
				n, ok := g.localNeedingEqual(t)
				if ok && !eq[n] {
					eq[n] = true
					changed = true
				}
			}
		}
	}

	return sortedKeys(eq)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

func componentTypes(t types.Type) []types.Type {
	switch u := t.(type) {
	case *types.Struct:
		out := make([]types.Type, 0, u.NumFields())
		for i := 0; i < u.NumFields(); i++ {
			out = append(out, u.Field(i).Type())
		}

		return out
	case *types.Slice:
		return []types.Type{u.Elem()}
	default:
		return nil
	}
}

func (g *generator) localNeedingEqual(t types.Type) (string, bool) {
	for {
		t = types.Unalias(t)

		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()

			continue
		}

		if s, ok := t.(*types.Slice); ok {
			t = s.Elem()

			continue
		}

		break
	}

	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() != g.pkg || n.TypeArgs().Len() > 0 || g.hasEqual(n) || refFree(n) {
		return "", false
	}

	switch n.Underlying().(type) {
	case *types.Struct, *types.Slice:
		return n.Obj().Name(), true
	default:
		return "", false
	}
}

func (g *generator) localNamed(name string) (*types.Named, bool) {
	nt, ok := g.pkg.Scope().Lookup(name).(*types.TypeName)
	if !ok || nt.IsAlias() {
		return nil, false
	}

	named, ok := nt.Type().(*types.Named)
	if !ok || named.TypeParams().Len() > 0 {
		return nil, false
	}

	return named, true
}

func (g *generator) hasEqual(n *types.Named) bool {
	if n.Obj().Pkg() == g.pkg && g.eqTypes[n.Obj().Name()] {
		return true
	}

	sel := types.NewMethodSet(types.NewPointer(n)).Lookup(n.Obj().Pkg(), "Equal")
	if sel == nil {
		return false
	}

	sig, ok := sel.Type().(*types.Signature)
	if !ok || sig.Params().Len() != 1 || sig.Results().Len() != 1 {
		return false
	}

	res, ok := sig.Results().At(0).Type().(*types.Basic)

	return ok && res.Kind() == types.Bool && types.Identical(sig.Params().At(0).Type(), n)
}

func refFree(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Kind() != types.UnsafePointer && u.Kind() != types.UntypedNil
	case *types.Array:
		return refFree(u.Elem())
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if !refFree(u.Field(i).Type()) {
				return false
			}
		}

		return true
	default:
		return false
	}
}

func (g *generator) emitEqual(name string) error {
	named, _ := g.localNamed(name)
	r := &g.buf
	recv := g.receiver(name)

	other := "y"
	if recv == other {
		other = "z"
	}

	if refFree(named) {
		fmt.Fprintf(r, "func (%s %s) Equal(%s %s) bool {\n\treturn %s == %s\n}\n\n", recv, name, other, name, recv, other)

		return nil
	}

	switch u := named.Underlying().(type) {
	case *types.Struct:
		var conds []string

		for i := 0; i < u.NumFields(); i++ {
			f := u.Field(i)
			if f.Name() == "_" {
				continue
			}

			a, b := recv+"."+f.Name(), other+"."+f.Name()

			cond, err := g.equalExpr(f.Type(), a, b)
			if err != nil {
				return fmt.Errorf("field %s: %w", f.Name(), err)
			}

			if g.absentWhenNil(f, u.Tag(i)) {
				cond = fmt.Sprintf("(%s == nil) == (%s == nil) && %s", a, b, cond)
			}

			conds = append(conds, cond)
		}

		if len(conds) == 0 {
			conds = []string{"true"}
		}

		fmt.Fprintf(r, "func (%s %s) Equal(%s %s) bool {\n\treturn %s\n}\n\n", recv, name, other, name, strings.Join(conds, " &&\n\t\t"))
	case *types.Slice:
		cond, err := g.equalExpr(u.Elem(), recv+"[i]", other+"[i]")
		if err != nil {
			return err
		}

		fmt.Fprintf(r, "func (%s %s) Equal(%s %s) bool {\n", recv, name, other, name)
		fmt.Fprintf(r, "\tif len(%s) != len(%s) {\n\t\treturn false\n\t}\n", recv, other)
		fmt.Fprintf(r, "\tfor i := range %s {\n\t\tif !(%s) {\n\t\t\treturn false\n\t\t}\n\t}\n", recv, cond)
		fmt.Fprintf(r, "\treturn true\n}\n\n")
	default:
		return fmt.Errorf("no Equal for %s", g.goTypeString(named))
	}

	return nil
}

func (g *generator) absentWhenNil(f *types.Var, rawTag string) bool {
	if _, ok := types.Unalias(f.Type()).Underlying().(*types.Slice); !ok {
		return false
	}

	tag, err := ParseTag(structTagGet(rawTag, "per"))

	return err == nil && tag.Optional
}

func (g *generator) equalExpr(t types.Type, a, b string) (string, error) {
	t = types.Unalias(t)

	if n, ok := t.(*types.Named); ok && g.hasEqual(n) {
		return a + ".Equal(" + b + ")", nil
	}

	if refFree(t) && types.Comparable(t) {
		return a + " == " + b, nil
	}

	switch u := t.Underlying().(type) {
	case *types.Pointer:
		inner, err := g.equalExpr(u.Elem(), "(*"+a+")", "*"+b)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("(%s == nil) == (%s == nil) && (%s == nil || %s)", a, b, a, inner), nil
	case *types.Slice:
		elem := types.Unalias(u.Elem())
		if refFree(elem) && types.Comparable(elem) {
			g.imports = appendUnique(g.imports, "slices")

			return "slices.Equal(" + a + ", " + b + ")", nil
		}

		if n, ok := elem.(*types.Named); ok && n.Obj().Pkg() == g.pkg && g.eqTypes[n.Obj().Name()] {
			g.imports = appendUnique(g.imports, "slices")

			return "slices.EqualFunc(" + a + ", " + b + ", " + n.Obj().Name() + ".Equal)", nil
		}

		inner, err := g.equalExpr(elem, "v1", "v2")
		if err != nil {
			return "", err
		}

		elemStr, err := g.typeRef(elem)
		if err != nil {
			return "", err
		}

		g.imports = appendUnique(g.imports, "slices")

		return fmt.Sprintf("slices.EqualFunc(%s, %s, func(v1, v2 %s) bool { return %s })", a, b, elemStr, inner), nil
	default:
		return "", fmt.Errorf("%s has no Equal method and holds references, so == does not compare its value", g.goTypeString(t))
	}
}

func (g *generator) typeRef(t types.Type) (string, error) {
	var foreign []string

	s := types.TypeString(t, func(p *types.Package) string {
		switch {
		case g.pkg != nil && p.Path() == g.pkg.Path():
			return ""
		case p.Path() == apImportPath:
			g.usesAP = true
		case p.Path() != perImportPath:
			foreign = append(foreign, p.Path())
		}

		return p.Name()
	})

	if len(foreign) > 0 {
		return "", fmt.Errorf("%s names package %s, which per_gen.go does not import", s, foreign[0])
	}

	return s, nil
}

func appendUnique(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}

	return append(list, s)
}
