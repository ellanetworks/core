// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"go/types"
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
	extSeq bool
}

// parseStruct classifies each field for emission. Fields tagged `per:"-"` or
// `per:"extseq"` are excluded from the field list.
func (g *generator) parseStruct(name string, named *types.Named, s *types.Struct) (*structType, error) {
	st := &structType{name: name, named: named}

	for i := 0; i < s.NumFields(); i++ {
		rawTag := s.Tag(i)

		perTag := structTagGet(rawTag, "per")
		if perTag == "-" {
			continue
		}

		parsed, err := ParseTag(perTag)
		if err != nil {
			return nil, err
		}

		if parsed.ExtSeq {
			st.extSeq = true
			continue
		}

		fi, err := g.classifyField(s.Field(i), rawTag)
		if err != nil {
			return nil, err
		}

		fi.fieldIdx = len(st.fields)
		st.fields = append(st.fields, fi)
	}

	return st, nil
}

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

	enumRoot int64
	enumExt  bool

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
	isSkip     bool
	// noDeref marks an OPTIONAL field whose absence is a nil slice rather than
	// a nil pointer, so the value is used without dereferencing.
	noDeref bool

	typeStr string
}

func (g *generator) classifyField(f *types.Var, rawTag string) (fieldInfo, error) {
	fi := fieldInfo{name: f.Name(), typeStr: g.goTypeString(f.Type())}
	fi.tag, fi.has = perTagFromField(f, rawTag)

	ft := f.Type()

	if elem, ok := isPointer(ft); ok {
		fi.isOptional = true
		ft = elem
		fi.typeStr = g.goTypeString(ft)
	}

	if fi.has && fi.tag.Skip {
		fi.isSkip = true
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
	b, ok := t.(*types.Basic)
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
	n, ok := t.(*types.Named)
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
// type such as `type TransportLayerAddress []byte` qualifies.
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
		if g.pkg != nil && p.Path() == g.pkg.Types.Path() {
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
