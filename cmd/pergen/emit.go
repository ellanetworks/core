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

// ---- Field classification ---------------------------------------------------

// structType holds a parsed Go struct with classified fields.
type structType struct {
	name   string
	named  *types.Named
	fields []fieldInfo
	// extSeq is true when the struct has a placeholder field tagged
	// `per:"extseq"`, marking the type as extensible ("...") without any
	// actual extension additions.
	extSeq bool
}

// parseStruct inspects a Go struct type, classifies each field, and returns a
// structType ready for code emission. Fields with `per:"-"` are skipped.
// Fields with `per:"extseq"` are placeholder markers that set the extSeq flag
// and are not included in the field list.
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

// fieldKind classifies a struct field for PER emission.
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
)

// fieldInfo holds the parsed classification of one struct field.
type fieldInfo struct {
	name string
	tag  FieldTag
	has  bool // whether a per: tag was present

	fieldIdx int // unique index within the struct for variable name uniqueness

	kind fieldKind

	// For kindConstrainedInt / kindUnconstrainedInt:
	boundsExpr string // e.g. `per.Bounds{LB: 0, UB: 255, HasLB: true, HasUB: true}`

	// For kindOctetString / kindBitString:
	sizeLB, sizeUB       int64
	hasSizeLB, hasSizeUB bool
	sizeExt              bool

	// For kindSequenceOf / kindDelegate:
	elemType types.Type
	// For kindDelegate (value or pointer receiver):
	delegateNamed   *types.Named
	delegateIsValue bool // true: value receiver MarshalPER; false: pointer

	// Whether this field is OPTIONAL (pointer) in the SEQUENCE.
	isOptional bool
	// Whether this field has a DEFAULT value.
	hasDefault bool

	// Go type spelling (for local variables in decode).
	typeStr string
}

// classifyField inspects a struct field and its tag, returning an emission plan.
func (g *generator) classifyField(f *types.Var, rawTag string) (fieldInfo, error) {
	fi := fieldInfo{name: f.Name(), typeStr: g.goTypeString(f.Type())}
	fi.tag, fi.has = perTagFromField(f, rawTag)

	ft := f.Type()

	// OPTIONAL: pointer field.
	if elem, ok := isPointer(ft); ok {
		fi.isOptional = true
		ft = elem
		fi.typeStr = g.goTypeString(ft)
	}

	// DEFAULT.
	if fi.has && fi.tag.DefaultExpr != "" {
		fi.hasDefault = true
	}

	switch {
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
		// Named integer type: use range tag if present, else unconstrained.
		fi.boundsExpr = g.boundsExprFromTag(fi.tag)
		if fi.boundsExpr != "" {
			fi.kind = kindConstrainedInt
		} else {
			fi.kind = kindUnconstrainedInt
			fi.boundsExpr = "per.Bounds{}"
		}
	case isString(ft):
		fi.kind = kindString
	case isFloat64(ft):
		fi.kind = kindREAL
	default:
		// Try named type with generated methods (delegate).
		if named, ok := isNamed(ft); ok {
			if g.implementsMarshalPER(named) {
				fi.kind = kindDelegate
				fi.delegateNamed = named
				fi.delegateIsValue = true
			}
		}
		// Try slice of delegatable type (SEQUENCE OF).
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

// boundsExprFromTag builds a per.Bounds literal from a range tag, or "" if no
// range constraint is present.
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

// isBasicInt reports whether t is a predeclared integer basic type.
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

// ---- Helpers for type inspection -------------------------------------------

// perTagFromField extracts the `per:` key from a struct field's tag.
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
		// Skip leading spaces.
		i := 0
		for i < len(rawTag) && rawTag[i] == ' ' {
			i++
		}

		rawTag = rawTag[i:]
		if rawTag == "" {
			break
		}
		// Scan to colon.
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

// isBool reports whether t is the predeclared bool type.
func isBool(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.Bool
}

// isString reports whether t is the predeclared string type.
func isString(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.String
}

// isFloat64 reports whether t is float64.
func isFloat64(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.Float64
}

// isNamedInt reports whether t is a named type whose underlying is a basic
// integer kind. Returns the named type or nil.
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

// isByteSlice reports whether t is []byte or []uint8.
func isByteSlice(t types.Type) bool {
	s, ok := t.(*types.Slice)
	if !ok {
		return false
	}

	b, ok := s.Elem().(*types.Basic)

	return ok && b.Kind() == types.Byte
}

// isBoolSlice reports whether t is []bool.
func isBoolSlice(t types.Type) bool {
	s, ok := t.(*types.Slice)
	if !ok {
		return false
	}

	return isBool(s.Elem())
}

// isPointer reports whether t is a pointer and returns the element type.
func isPointer(t types.Type) (elem types.Type, ok bool) {
	p, ok := t.(*types.Pointer)
	if !ok {
		return nil, false
	}

	return p.Elem(), true
}

// isSlice reports whether t is a slice and returns the element type.
func isSlice(t types.Type) (elem types.Type, ok bool) {
	s, ok := t.(*types.Slice)
	if !ok {
		return nil, false
	}

	return s.Elem(), true
}

// isNamed reports whether t is a named type (returns it).
func isNamed(t types.Type) (*types.Named, bool) {
	n, ok := t.(*types.Named)
	return n, ok
}

// implementsMarshalPER reports whether the named type has (or will have) a
// MarshalPER method — either already present in source or generated by pergen.
func (g *generator) implementsMarshalPER(named *types.Named) bool {
	if hasValueMethod(named, "MarshalPER") || hasPointerMethod(named, "MarshalPER") {
		return true
	}

	return g.willGenerate(named)
}

// implementsUnmarshalPER reports whether *named has (or will have) an
// UnmarshalPER method.
func (g *generator) implementsUnmarshalPER(named *types.Named) bool {
	if hasPointerMethod(named, "UnmarshalPER") {
		return true
	}

	return g.willGenerate(named)
}

// willGenerate reports whether the named type is in the generator's set of
// types that will get generated methods in this run.
func (g *generator) willGenerate(named *types.Named) bool {
	if g.genTypes == nil {
		return false
	}

	return g.genTypes[named.Obj().Name()]
}

func hasValueMethod(named *types.Named, name string) bool {
	ms := types.NewMethodSet(named)
	for method := range ms.Methods() {
		if method.Obj().Name() == name {
			return true
		}
	}

	return false
}

func hasPointerMethod(named *types.Named, name string) bool {
	ms := types.NewMethodSet(types.NewPointer(named))
	for method := range ms.Methods() {
		if method.Obj().Name() == name {
			return true
		}
	}

	return false
}

// goTypeString returns the Go source spelling of a types.Type in the current
// package (unqualified for same-package named types).
func (g *generator) goTypeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if g.pkg != nil && p.Path() == g.pkg.Types.Path() {
			return ""
		}

		return p.Name()
	})
}

// receiverName returns a short receiver variable name for a type name.
func receiverName(typeName string) string {
	if typeName == "" {
		return "v"
	}

	r, size := utf8.DecodeRuneInString(typeName)
	lo := unicode.ToLower(r)

	return string(lo) + typeName[size:]
}

// perImportPath is the import path of the runtime per package.
const perImportPath = "github.com/ellanetworks/core/internal/per"
