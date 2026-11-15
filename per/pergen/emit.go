// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pergen

import (
	"errors"
	"fmt"
	"go/types"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type structType struct {
	name   string
	named  *types.Named
	fields []fieldInfo
	// extSeq marks the type extensible ("...") with no extension additions.
	extSeq     bool
	extRaw     string
	extIEs     []fieldInfo
	extAlt     string
	choiceCrit string
}

// parseStruct classifies each field for emission. Fields tagged `per:"-"` or
// `per:"extseq"` are excluded from the field list.
func (g *generator) parseStruct(name string, named *types.Named, s *types.Struct) (*structType, error) {
	st := &structType{name: name, named: named}

	for i := 0; i < s.NumFields(); i++ {
		rawTag := s.Tag(i)
		f := s.Field(i)

		perTag := structTagGet(rawTag, "per")
		if perTag == "-" {
			continue
		}

		parsed, err := ParseTag(perTag)
		if err != nil {
			return nil, err
		}

		switch {
		case parsed.ExtSeq:
			if g.apMode() {
				return nil, errors.New("extseq drops extension additions: use an extraw field (SEQUENCE) or an extalt field (CHOICE)")
			}

			st.extSeq = true

			continue
		case parsed.ExtRaw:
			if st.extRaw != "" {
				return nil, fmt.Errorf("%s: more than one extraw field", name)
			}

			if err := g.requireType(f, perImportPath, "ExtensionAdditions", false); err != nil {
				return nil, err
			}

			st.extRaw = f.Name()

			continue
		case parsed.ExtAlt:
			if err := g.requireAP(f, "extalt"); err != nil {
				return nil, err
			}

			if err := g.requireType(f, perImportPath, "ExtensionAlternative", true); err != nil {
				return nil, err
			}

			st.extAlt = f.Name()

			continue
		case parsed.ChoiceCrit:
			if err := g.requireAP(f, "choicecrit"); err != nil {
				return nil, err
			}

			if err := g.requireType(f, apImportPath, "WireCriticality", false); err != nil {
				return nil, err
			}

			st.choiceCrit = f.Name()

			continue
		case parsed.ExtTable != "":
			if g.apMode() {
				return nil, fmt.Errorf("field %s: exttable:%s names a hand-written table: tag the extensions with extie and use a bare exttable", f.Name(), parsed.ExtTable)
			}

			st.fields = append(st.fields, fieldInfo{name: f.Name(), tag: parsed, extTable: parsed.ExtTable, fieldIdx: len(st.fields)})

			continue
		case parsed.ExtTableGenerated:
			if err := g.requireAP(f, "exttable"); err != nil {
				return nil, err
			}

			if err := g.requireType(f, apImportPath, "Preserved", false); err != nil {
				return nil, err
			}

			st.fields = append(st.fields, fieldInfo{name: f.Name(), tag: parsed, extTable: extTableName(name), genTable: true, fieldIdx: len(st.fields)})

			continue
		}

		if parsed.ExtIE && parsed.ChoiceIdx >= 0 && parsed.ExtIEID == "" {
			if err := g.requireAP(f, "extie"); err != nil {
				return nil, err
			}

			if err := g.requireType(f, apImportPath, "SingleContainer", true); err != nil {
				return nil, err
			}

			st.fields = append(st.fields, fieldInfo{name: f.Name(), tag: parsed, has: true, rawSingle: true, isOptional: true, fieldIdx: len(st.fields)})

			continue
		}

		fi, err := g.classifyField(f, rawTag)
		if err != nil {
			return nil, err
		}

		if parsed.ExtIE {
			if err := g.requireAP(f, "extie"); err != nil {
				return nil, err
			}

			if err := checkExtIE(f, parsed); err != nil {
				return nil, err
			}

			if parsed.ChoiceIdx < 0 {
				if !fi.isOptional {
					if _, ok := f.Type().Underlying().(*types.Slice); !ok {
						return nil, fmt.Errorf("field %s: an extie extension must be a pointer or a slice", f.Name())
					}

					fi.isOptional = true
					fi.noDeref = true
				}

				fi.fieldIdx = len(st.fields) + len(st.extIEs) + 1000
				st.extIEs = append(st.extIEs, fi)

				continue
			}
		}

		if g.apMode() && fi.kind == kindEnum && fi.enumExt {
			return nil, fmt.Errorf("field %s: an extensible ENUMERATED needs its own per.Enum type, not an inline tag", f.Name())
		}

		if g.apMode() && fi.tag.Ext && fi.tag.ChoiceIdx < 0 {
			return nil, fmt.Errorf("field %s: SEQUENCE extension additions are kept raw: use an extraw field", f.Name())
		}

		fi.fieldIdx = len(st.fields)
		st.fields = append(st.fields, fi)
	}

	if st.extIEs != nil && !slices.ContainsFunc(st.fields, func(fi fieldInfo) bool { return fi.genTable }) {
		return nil, errors.New("extie extensions need an exttable field")
	}

	return st, nil
}

func checkExtIE(f *types.Var, t FieldTag) error {
	if t.ChoiceIdx >= 0 && t.ExtIEID == "" {
		return nil
	}

	if t.ExtIEID == "" || t.Criticality == "" {
		return fmt.Errorf("field %s: extie needs an id and a crit", f.Name())
	}

	if t.Presence == "conditional" && t.Condition == "" {
		return fmt.Errorf("field %s: a conditional extension needs cond", f.Name())
	}

	if t.Condition != "" && t.Presence != "conditional" {
		return fmt.Errorf("field %s: cond applies only to presence:conditional", f.Name())
	}

	return nil
}

func (g *generator) requireAP(f *types.Var, tag string) error {
	if !g.apMode() {
		return fmt.Errorf("field %s: %s needs application protocol mode (a profile and a ProtocolIE-ID type)", f.Name(), tag)
	}

	return nil
}

func (g *generator) requireType(f *types.Var, pkgPath, name string, pointer bool) error {
	t := types.Unalias(f.Type())

	if pointer {
		p, ok := t.(*types.Pointer)
		if !ok {
			return fmt.Errorf("field %s: want *%s", f.Name(), name)
		}

		t = types.Unalias(p.Elem())
	}

	n, ok := t.(*types.Named)
	if !ok || n.Origin().Obj().Pkg() == nil || n.Origin().Obj().Pkg().Path() != pkgPath || n.Origin().Obj().Name() != name {
		return fmt.Errorf("field %s: want %s.%s, got %s", f.Name(), pkgPath, name, g.goTypeString(f.Type()))
	}

	return nil
}

func extTableName(typeName string) string { return "extIEs" + typeName }

type fieldKind int

const (
	kindUnsupported      fieldKind = iota
	kindBool                       // BOOLEAN
	kindConstrainedInt             // INTEGER with range
	kindUnconstrainedInt           // INTEGER without range
	kindOctetString                // []byte with size
	kindBitString                  // []bool with size
	kindSequenceOf                 // []T where T has MarshalPER
	kindDelegate                   // named type with MarshalPER/UnmarshalPER
	kindString                     // string (UTF8String etc.)
	kindREAL                       // float64
	kindEnum                       // int-typed field tagged ENUMERATED
)

type fieldInfo struct {
	name string
	tag  FieldTag
	has  bool // whether a per: tag was present

	fieldIdx int // index within the struct, used to make local variable names unique

	kind fieldKind

	boundsExpr string // e.g. `per.Bounds{LB: 0, UB: 255, HasLB: true, HasUB: true}`

	enumRoot  int64
	enumExt   bool
	enumTotal int64

	// charTypeExpr is the per.Char* constant for a known-multiplier string
	// type, or "" for UTF8String and untyped strings.
	charTypeExpr string

	sizeLB, sizeUB       int64
	hasSizeLB, hasSizeUB bool
	sizeExt              bool

	elemType        types.Type
	delegateNamed   *types.Named
	delegateIsValue bool // value receiver MarshalPER, else pointer receiver

	isOptional bool
	hasDefault bool
	extTable   string
	genTable   bool
	rawSingle  bool
	// noDeref marks an OPTIONAL field whose absence is a nil slice rather than
	// a nil pointer, so the value is used without dereferencing.
	noDeref bool

	typeStr string
}

func (g *generator) classifyField(f *types.Var, rawTag string) (fieldInfo, error) {
	fi := fieldInfo{name: f.Name(), typeStr: g.goTypeString(f.Type())}
	fi.tag, fi.has = perTagFromField(f, rawTag)

	ft := types.Unalias(f.Type())

	if elem, ok := isPointer(ft); ok {
		fi.isOptional = true
		fi.typeStr = g.goTypeString(elem)
		ft = types.Unalias(elem)
	}

	// A non-pointer OPTIONAL expresses absence as nil-ness, so the Go type must
	// be nilable; anything else would emit an `x != nil` that does not compile.
	if fi.has && fi.tag.Optional && !fi.isOptional {
		if !isNilable(ft) {
			return fi, fmt.Errorf("field %s: `optional` requires a pointer or slice type, got %s",
				f.Name(), g.goTypeString(ft))
		}

		fi.isOptional = true
		fi.noDeref = true
	}

	if fi.has && fi.tag.DefaultExpr != "" {
		fi.hasDefault = true
	}

	if named, ok := isNamed(ft); ok && g.isEnum(named) {
		if fi.has && fi.tag.Name == "ENUMERATED" {
			return fi, fmt.Errorf("field %s: %s is a per.Enum type: drop the ENUMERATED tag", f.Name(), named.Obj().Name())
		}

		fi.kind = kindDelegate
		fi.delegateNamed = named
		fi.delegateIsValue = true

		return fi, nil
	}

	// A named type with a hand-written MarshalPER always delegates, even when
	// its underlying type would classify as a primitive (e.g. a semantic
	// integer encoded as an OCTET STRING on the wire).
	if named, ok := isNamed(ft); ok && !g.willGenerate(named) && g.implementsMarshalPER(named) {
		fi.kind = kindDelegate
		fi.delegateNamed = named
		fi.delegateIsValue = true

		return fi, nil
	}

	switch {
	case fi.has && fi.tag.Name == "ENUMERATED":
		if !isBasicInt(ft) && isNamedInt(ft) == nil {
			return fi, fmt.Errorf("field %s: ENUMERATED requires an integer type, got %s", f.Name(), g.goTypeString(ft))
		}

		if !fi.tag.HasRangeUB || (fi.tag.HasRangeLB && fi.tag.RangeLB != 0) {
			return fi, fmt.Errorf("field %s: ENUMERATED requires range:0..N for N+1 root values", f.Name())
		}

		fi.kind = kindEnum
		fi.enumRoot = fi.tag.RangeUB + 1
		fi.enumExt = fi.tag.RangeExtensible

		if fi.tag.HasEnumExtValues {
			if !fi.enumExt {
				return fi, fmt.Errorf("field %s: extvalues requires an extensible ENUMERATED range", f.Name())
			}

			fi.enumTotal = fi.enumRoot + fi.tag.EnumExtValues
		}
	case isBool(ft):
		fi.kind = kindBool
	case isByteSlice(ft):
		fi.kind = kindOctetString
		if fi.has {
			fi.sizeLB, fi.sizeUB = fi.tag.SizeLB, fi.tag.SizeUB
			fi.hasSizeLB, fi.hasSizeUB = fi.tag.HasSizeLB, fi.tag.HasSizeUB
			fi.sizeExt = fi.tag.SizeExtensible
		}
	case isBoolSlice(ft):
		fi.kind = kindBitString
		if fi.has {
			fi.sizeLB, fi.sizeUB = fi.tag.SizeLB, fi.tag.SizeUB
			fi.hasSizeLB, fi.hasSizeUB = fi.tag.HasSizeLB, fi.tag.HasSizeUB
			fi.sizeExt = fi.tag.SizeExtensible
		}
	case isBasicInt(ft):
		fi.boundsExpr = g.boundsExprFromTag(fi.tag)
		if fi.boundsExpr != "" {
			fi.kind = kindConstrainedInt
		} else {
			fi.kind = kindUnconstrainedInt
			fi.boundsExpr = "per.Bounds{}"
		}
	case isNamedInt(ft) != nil:
		fi.boundsExpr = g.boundsExprFromTag(fi.tag)
		if fi.boundsExpr != "" {
			fi.kind = kindConstrainedInt
		} else {
			fi.kind = kindUnconstrainedInt
			fi.boundsExpr = "per.Bounds{}"
		}
	case isString(ft):
		fi.kind = kindString
		if fi.has {
			fi.sizeLB, fi.sizeUB = fi.tag.SizeLB, fi.tag.SizeUB
			fi.hasSizeLB, fi.hasSizeUB = fi.tag.HasSizeLB, fi.tag.HasSizeUB
			fi.sizeExt = fi.tag.SizeExtensible
			fi.charTypeExpr = charTypeExprFromName(fi.tag.Name)
		}
	case isFloat64(ft):
		fi.kind = kindREAL
	default:
		if named, ok := isNamed(ft); ok {
			if g.implementsMarshalPER(named) {
				fi.kind = kindDelegate
				fi.delegateNamed = named
				fi.delegateIsValue = true
			}
		}

		if elem, ok := isSlice(ft); ok {
			if enamed, ok2 := isNamed(elem); ok2 && g.implementsMarshalPER(enamed) {
				fi.kind = kindSequenceOf

				fi.elemType = elem
				if fi.has {
					fi.sizeLB, fi.sizeUB = fi.tag.SizeLB, fi.tag.SizeUB
					fi.hasSizeLB, fi.hasSizeUB = fi.tag.HasSizeLB, fi.tag.HasSizeUB
				}
			} else if enamed != nil && g.implementsUnmarshalPER(enamed) {
				fi.kind = kindSequenceOf

				fi.elemType = elem
				if fi.has {
					fi.sizeLB, fi.sizeUB = fi.tag.SizeLB, fi.tag.SizeUB
					fi.hasSizeLB, fi.hasSizeUB = fi.tag.HasSizeLB, fi.tag.HasSizeUB
				}
			}
		}
	}

	if fi.kind == kindUnsupported {
		return fi, fmt.Errorf("field %s: unsupported type %s", f.Name(), g.goTypeString(ft))
	}

	return fi, nil
}

// charTypeExprFromName returns "" for UTF8String and unnamed strings, which
// encode 8 bits per character rather than a known multiplier.
func charTypeExprFromName(name string) string {
	switch name {
	case "NumericString":
		return "per.CharNumericString"
	case "PrintableString":
		return "per.CharPrintableString"
	case "VisibleString":
		return "per.CharVisibleString"
	case "IA5String":
		return "per.CharIA5String"
	case "BMPString":
		return "per.CharBMPString"
	case "UniversalString":
		return "per.CharUniversalString"
	default:
		return ""
	}
}

// boundsExprFromTag returns "" when the tag carries no range constraint.
func (g *generator) boundsExprFromTag(tag FieldTag) string {
	if !tag.HasRangeLB && !tag.HasRangeUB && !tag.RangeExtensible {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("per.Bounds{")

	if tag.HasRangeLB {
		sb.WriteString("LB: ")
		sb.WriteString(strconv.FormatInt(tag.RangeLB, 10))
		sb.WriteString(", HasLB: true, ")
	}

	if tag.HasRangeUB {
		sb.WriteString("UB: ")
		sb.WriteString(strconv.FormatInt(tag.RangeUB, 10))
		sb.WriteString(", HasUB: true, ")
	}

	if tag.RangeExtensible {
		sb.WriteString("Extensible: true, ")
	}

	sb.WriteString("}")

	return sb.String()
}

func isBasicInt(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	if !ok {
		return false
	}

	switch b.Kind() {
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
		types.Uintptr:
		return true
	default:
		return false
	}
}

func perTagFromField(_ *types.Var, rawTag string) (FieldTag, bool) {
	tag := structTagGet(rawTag, "per")
	if tag == "" {
		return FieldTag{}, false
	}

	parsed, err := ParseTag(tag)
	if err != nil {
		return FieldTag{}, false
	}

	return parsed, true
}

// structTagGet reimplements reflect.StructTag.Get without reflect.
func structTagGet(rawTag, key string) string {
	for rawTag != "" {
		i := 0
		for i < len(rawTag) && rawTag[i] == ' ' {
			i++
		}

		rawTag = rawTag[i:]
		if rawTag == "" {
			break
		}

		i = 0
		for i < len(rawTag) && rawTag[i] > ' ' && rawTag[i] != ':' && rawTag[i] != '"' {
			i++
		}

		if i+1 >= len(rawTag) || rawTag[i] != ':' || rawTag[i+1] != '"' {
			break
		}

		name := rawTag[:i]
		rawTag = rawTag[i+2:]

		i = 0
		for i < len(rawTag) && rawTag[i] != '"' {
			if rawTag[i] == '\\' {
				i++
			}

			i++
		}

		if i >= len(rawTag) {
			break
		}

		val := rawTag[:i]
		rawTag = rawTag[i+1:]

		if name == key {
			return val
		}
	}

	return ""
}

func isBool(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.Bool
}

func isString(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.String
}

func isFloat64(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.Float64
}

func isNamedInt(t types.Type) *types.Named {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil
	}

	b, ok := n.Underlying().(*types.Basic)
	if !ok {
		return nil
	}

	switch b.Kind() {
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
		types.Uintptr:
		return n
	default:
		return nil
	}
}

func isByteSlice(t types.Type) bool {
	s, ok := t.(*types.Slice)
	if !ok {
		return false
	}

	b, ok := s.Elem().(*types.Basic)

	return ok && b.Kind() == types.Byte
}

func isBoolSlice(t types.Type) bool {
	s, ok := t.(*types.Slice)
	if !ok {
		return false
	}

	return isBool(s.Elem())
}

func isPointer(t types.Type) (elem types.Type, ok bool) {
	p, ok := t.(*types.Pointer)
	if !ok {
		return nil, false
	}

	return p.Elem(), true
}

// isNilable judges named types by their underlying type, so a defined slice
// type such as `type NASPDU []byte` qualifies.
func isNilable(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Slice, *types.Pointer, *types.Map, *types.Interface:
		return true
	default:
		return false
	}
}

func isSlice(t types.Type) (elem types.Type, ok bool) {
	s, ok := t.(*types.Slice)
	if !ok {
		return nil, false
	}

	return s.Elem(), true
}

func isNamed(t types.Type) (*types.Named, bool) {
	n, ok := t.(*types.Named)
	return n, ok
}

func (g *generator) implementsMarshalPER(named *types.Named) bool {
	if hasDeclaredMethod(named, "MarshalPER") {
		return true
	}

	return g.willGenerate(named)
}

func (g *generator) implementsUnmarshalPER(named *types.Named) bool {
	if hasDeclaredMethod(named, "UnmarshalPER") {
		return true
	}

	return g.willGenerate(named)
}

func (g *generator) willGenerate(named *types.Named) bool {
	if g.genTypes == nil {
		return false
	}

	return g.genTypes[named.Obj().Name()]
}

// hasDeclaredMethod excludes methods promoted from an embedded type:
// delegating to a promoted codec would encode only the embedded field.
func hasDeclaredMethod(named *types.Named, name string) bool {
	for method := range named.Methods() {
		if method.Name() == name {
			return true
		}
	}

	return false
}

// goTypeString spells t as Go source, unqualified for same-package types.
func (g *generator) goTypeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if g.pkg != nil && p.Path() == g.pkg.Path() {
			return ""
		}

		return p.Name()
	})
}

func receiverName(typeName string) string {
	if typeName == "" {
		return "v"
	}

	r, size := utf8.DecodeRuneInString(typeName)
	lo := unicode.ToLower(r)

	return string(lo) + typeName[size:]
}

const perImportPath = "github.com/ellanetworks/core/per"
