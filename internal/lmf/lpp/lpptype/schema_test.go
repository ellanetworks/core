// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
)

type asnType struct {
	Ref       string        `json:"ref"`
	Kind      string        `json:"kind"`
	Ext       any           `json:"ext"`
	Additions []asnAddition `json:"additions"`
	Comps     []asnComp     `json:"comps"`
	Alts      []asnComp     `json:"alts"`
	Elem      *asnType      `json:"elem"`
	Range     []int64       `json:"range"`
	Size      []int64       `json:"size"`
	Root      []string      `json:"root"`
	Names     []string      `json:"names"`
}

type asnComp struct {
	Name     string  `json:"name"`
	Optional bool    `json:"optional"`
	Default  bool    `json:"default"`
	Type     asnType `json:"type"`
}

type asnAddition struct {
	Group bool         `json:"group"`
	Comps []asnExtComp `json:"comps"`
}

type asnExtComp struct {
	Name string   `json:"name"`
	Type *asnType `json:"type"`
}

type asnSchema struct {
	Root  string             `json:"root"`
	Types map[string]asnType `json:"types"`
}

type goTag struct {
	name       string
	lb, ub     *int64
	extensible bool
	extValues  *int64
	sizeLB     *int64
	sizeUB     *int64
	optional   bool
	hasDefault bool
	choice     int
}

func parseGoTag(raw string) goTag {
	t := goTag{choice: -1}
	parts := strings.Split(raw, ",")

	for i := 0; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])

		switch {
		case i == 0 && p != "" && !strings.Contains(p, ":") && p != "optional":
			t.name = p
		case p == "optional":
			t.optional = true
		case p == "...":
			t.extensible = true
		case strings.HasPrefix(p, "range:"):
			t.lb, t.ub = parseGoRange(strings.TrimPrefix(p, "range:"))
		case strings.HasPrefix(p, "size:"):
			t.sizeLB, t.sizeUB = parseGoRange(strings.TrimPrefix(p, "size:"))
		case strings.HasPrefix(p, "extvalues:"):
			n, _ := strconv.ParseInt(strings.TrimPrefix(p, "extvalues:"), 10, 64)
			t.extValues = &n
		case strings.HasPrefix(p, "default:"):
			t.hasDefault = true
		case strings.HasPrefix(p, "choice:"):
			t.choice, _ = strconv.Atoi(strings.TrimPrefix(p, "choice:"))
		}
	}

	return t
}

func parseGoRange(s string) (*int64, *int64) {
	lo, hi, ok := strings.Cut(s, "..")
	if !ok {
		hi = lo
	}

	l, err1 := strconv.ParseInt(lo, 10, 64)
	u, err2 := strconv.ParseInt(hi, 10, 64)

	if err1 != nil || err2 != nil {
		return nil, nil
	}

	return &l, &u
}

var (
	asnReleaseSuffix = regexp.MustCompile(`-[rv][0-9]+[a-z0-9]*$`)
	goReleaseSuffix  = regexp.MustCompile(`([a-z])[RV][0-9]+$`)
)

func normalizeName(s string) string {
	s = asnReleaseSuffix.ReplaceAllString(s, "")
	s = goReleaseSuffix.ReplaceAllString(s, "$1")

	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		default:
			return -1
		}
	}, s)
}

type schemaChecker struct {
	t       *testing.T
	schema  asnSchema
	visited map[string]bool
	enums   map[reflect.Type]asnType
}

func (c *schemaChecker) resolve(a asnType) (asnType, string) {
	if a.Ref == "" {
		return a, ""
	}

	resolved, ok := c.schema.Types[a.Ref]
	if !ok {
		c.t.Fatalf("schema has no type %s", a.Ref)
	}

	return resolved, a.Ref
}

func (c *schemaChecker) errorf(path, format string, args ...any) {
	c.t.Errorf("%s: %s", path, fmt.Sprintf(format, args...))
}

func isNull(t reflect.Type) bool {
	return t == reflect.TypeFor[per.Null]()
}

type goField struct {
	name  string
	typ   reflect.Type
	tag   goTag
	isPtr bool
}

var unmodelledType = reflect.TypeFor[UnmodelledExtension]()

func isExtensionAddition(sf reflect.StructField) bool {
	return slices.Contains(strings.Split(sf.Tag.Get("per"), ","), "ext")
}

func extensionFields(t reflect.Type) []goField {
	var fields []goField

	for i := range t.NumField() {
		sf := t.Field(i)
		if isExtensionAddition(sf) {
			fields = append(fields, goField{name: sf.Name, typ: sf.Type.Elem(), isPtr: true})
		}
	}

	return fields
}

func structFields(t reflect.Type) ([]goField, bool) {
	var (
		fields     []goField
		extensible bool
	)

	for i := range t.NumField() {
		sf := t.Field(i)
		raw := sf.Tag.Get("per")

		if raw == "extseq" {
			extensible = true
			continue
		}

		if isExtensionAddition(sf) {
			continue
		}

		ft := sf.Type
		isPtr := ft.Kind() == reflect.Pointer

		if isPtr {
			ft = ft.Elem()
		}

		fields = append(fields, goField{name: sf.Name, typ: ft, tag: parseGoTag(raw), isPtr: isPtr})
	}

	return fields, extensible
}

func (c *schemaChecker) check(path string, gt reflect.Type, tag goTag, a asnType) {
	a, ref := c.resolve(a)

	key := gt.String() + "=" + ref
	if ref != "" {
		if c.visited[key] {
			return
		}

		c.visited[key] = true
	}

	switch a.Kind {
	case "SEQUENCE":
		c.checkSequence(path, gt, a)
	case "CHOICE":
		c.checkChoice(path, gt, a)
	case "SEQUENCE OF":
		c.checkSequenceOf(path, gt, tag, a)
	case "INTEGER":
		c.checkInteger(path, gt, tag, a)
	case "ENUMERATED":
		c.checkEnumerated(path, gt, tag, a)
	case "BIT STRING":
		c.checkSize(path, gt, reflect.Bool, tag, a)
	case "OCTET STRING":
		c.checkSize(path, gt, reflect.Uint8, tag, a)
	case "VisibleString":
		if gt.Kind() != reflect.String || tag.name != "VisibleString" {
			c.errorf(path, "VisibleString modelled as %s tagged %q", gt, tag.name)
		}

		checkBounds(c, path, "size", tag.sizeLB, tag.sizeUB, a.Size)
	case "BOOLEAN":
		if gt.Kind() != reflect.Bool {
			c.errorf(path, "BOOLEAN modelled as %s", gt)
		}
	case "NULL":
		if !isNull(gt) {
			c.errorf(path, "NULL modelled as %s", gt)
		}
	default:
		c.errorf(path, "unhandled ASN.1 kind %s", a.Kind)
	}
}

func (c *schemaChecker) checkSequence(path string, gt reflect.Type, a asnType) {
	if len(a.Comps) == 0 && a.Ext == false {
		if !isNull(gt) {
			c.errorf(path, "empty SEQUENCE modelled as %s", gt)
		}

		return
	}

	if gt.Kind() != reflect.Struct {
		c.errorf(path, "SEQUENCE modelled as %s", gt)
		return
	}

	fields, extensible := structFields(gt)

	if extensible != (a.Ext != false) {
		c.errorf(path, "extensible = %t, want %t", extensible, a.Ext != false)
	}

	if len(fields) != len(a.Comps) {
		c.errorf(path, "%s has %d fields, want %d root components", gt.Name(), len(fields), len(a.Comps))
		return
	}

	for i, comp := range a.Comps {
		f := fields[i]
		fpath := path + "." + comp.Name

		if normalizeName(f.name) != normalizeName(comp.Name) {
			c.errorf(fpath, "field %d is %s.%s", i, gt.Name(), f.name)
		}

		optional := f.isPtr || f.tag.optional
		if optional != comp.Optional {
			c.errorf(fpath, "optional = %t, want %t", optional, comp.Optional)
		}

		if f.tag.hasDefault != comp.Default {
			c.errorf(fpath, "default = %t, want %t", f.tag.hasDefault, comp.Default)
		}

		c.check(fpath, f.typ, f.tag, comp.Type)
	}

	c.checkAdditions(path, gt, a)
}

func (c *schemaChecker) checkAdditions(path string, gt reflect.Type, a asnType) {
	fields := extensionFields(gt)

	if len(fields) > len(a.Additions) {
		c.errorf(path, "%s has %d extension additions, want at most %d", gt.Name(), len(fields), len(a.Additions))
		return
	}

	for i, f := range fields {
		addition := a.Additions[i]
		apath := fmt.Sprintf("%s.<addition %d>", path, i)

		if f.typ == unmodelledType {
			continue
		}

		if !addition.Group {
			c.checkExtComp(apath+"."+addition.Comps[0].Name, f.typ, goTag{choice: -1}, addition.Comps[0])
			continue
		}

		members, extensible := structFields(f.typ)
		if extensible || f.typ.Kind() != reflect.Struct {
			c.errorf(apath, "extension addition group modelled as %s", f.typ)
			continue
		}

		if len(members) != len(addition.Comps) {
			c.errorf(apath, "%s has %d fields, want %d group components", f.typ.Name(), len(members), len(addition.Comps))
			continue
		}

		for j, comp := range addition.Comps {
			m := members[j]
			mpath := apath + "." + comp.Name

			if normalizeName(m.name) != normalizeName(comp.Name) {
				c.errorf(mpath, "field %d is %s.%s", j, f.typ.Name(), m.name)
			}

			if !m.isPtr && !m.tag.optional {
				c.errorf(mpath, "extension addition group component is not optional")
			}

			if m.typ == unmodelledType {
				continue
			}

			c.checkExtComp(mpath, m.typ, m.tag, comp)
		}
	}
}

func (c *schemaChecker) checkExtComp(path string, gt reflect.Type, tag goTag, comp asnExtComp) {
	if comp.Type == nil {
		c.errorf(path, "modelled as %s but the schema does not describe it", gt)
		return
	}

	c.check(path, gt, tag, *comp.Type)
}

func (c *schemaChecker) checkChoice(path string, gt reflect.Type, a asnType) {
	fields, extensible := structFields(gt)

	if extensible != (a.Ext != false) {
		c.errorf(path, "extensible = %t, want %t", extensible, a.Ext != false)
	}

	if len(fields) != len(a.Alts) {
		c.errorf(path, "%s has %d alternatives, want %d", gt.Name(), len(fields), len(a.Alts))
		return
	}

	for i, alt := range a.Alts {
		f := fields[i]
		apath := path + "." + alt.Name

		if normalizeName(f.name) != normalizeName(alt.Name) {
			c.errorf(apath, "alternative %d is %s.%s", i, gt.Name(), f.name)
		}

		if f.tag.choice != i {
			c.errorf(apath, "choice index = %d, want %d", f.tag.choice, i)
		}

		c.check(apath, f.typ, f.tag, alt.Type)
	}
}

func (c *schemaChecker) checkSequenceOf(path string, gt reflect.Type, tag goTag, a asnType) {
	if gt.Kind() == reflect.Struct {
		fields, extensible := structFields(gt)
		if extensible || len(fields) != 1 || fields[0].typ.Kind() != reflect.Slice {
			c.errorf(path, "SEQUENCE OF modelled as %s", gt)
			return
		}

		gt, tag = fields[0].typ, fields[0].tag
	}

	if gt.Kind() != reflect.Slice || tag.name != "SEQUENCE-OF" {
		c.errorf(path, "SEQUENCE OF modelled as %s tagged %q", gt, tag.name)
		return
	}

	checkBounds(c, path, "size", tag.sizeLB, tag.sizeUB, a.Size)

	elem := gt.Elem()
	elemType, _ := c.resolve(*a.Elem)

	if elemType.Kind != "SEQUENCE" && elemType.Kind != "CHOICE" && elem.Kind() == reflect.Struct {
		fields, _ := structFields(elem)
		if len(fields) == 1 {
			c.check(path+"[]", fields[0].typ, fields[0].tag, *a.Elem)
			return
		}
	}

	c.check(path+"[]", elem, goTag{choice: -1}, *a.Elem)
}

func (c *schemaChecker) checkInteger(path string, gt reflect.Type, tag goTag, a asnType) {
	if gt.Kind() != reflect.Int64 {
		c.errorf(path, "INTEGER modelled as %s", gt)
	}

	if tag.extensible {
		c.errorf(path, "INTEGER tagged extensible")
	}

	checkBounds(c, path, "range", tag.lb, tag.ub, a.Range)
}

func (c *schemaChecker) checkEnumerated(path string, gt reflect.Type, tag goTag, a asnType) {
	if gt.Kind() != reflect.Int64 || gt.Name() == "int64" {
		c.errorf(path, "ENUMERATED modelled as %s, want a named integer type", gt)
	}

	if tag.name != "ENUMERATED" {
		c.errorf(path, "ENUMERATED tagged %q", tag.name)
	}

	if tag.lb == nil || *tag.lb != 0 || tag.ub == nil || *tag.ub != int64(len(a.Root))-1 {
		c.errorf(path, "range does not cover the %d root values", len(a.Root))
	}

	ext, extensible := a.Ext.([]any)
	if tag.extensible != extensible {
		c.errorf(path, "extensible = %t, want %t", tag.extensible, extensible)
	}

	if extensible && (tag.extValues == nil || *tag.extValues != int64(len(ext))) {
		c.errorf(path, "extvalues = %v, want %d", tag.extValues, len(ext))
	}

	if prev, ok := c.enums[gt]; ok && !slices.Equal(normalizedValues(prev), normalizedValues(a)) {
		c.errorf(path, "%s models two different enumerations", gt.Name())
	}

	c.enums[gt] = a
}

func (c *schemaChecker) checkSize(path string, gt reflect.Type, elem reflect.Kind, tag goTag, a asnType) {
	if gt.Kind() != reflect.Slice || gt.Elem().Kind() != elem {
		c.errorf(path, "%s modelled as %s", a.Kind, gt)
	}

	checkBounds(c, path, "size", tag.sizeLB, tag.sizeUB, a.Size)
}

func checkBounds(c *schemaChecker, path, what string, lb, ub *int64, want []int64) {
	if want == nil {
		if lb != nil || ub != nil {
			c.errorf(path, "%s is constrained, want unconstrained", what)
		}

		return
	}

	if lb == nil || ub == nil || *lb != want[0] || *ub != want[1] {
		c.errorf(path, "%s = %v..%v, want %d..%d", what, deref(lb), deref(ub), want[0], want[1])
	}
}

func deref(p *int64) any {
	if p == nil {
		return "unset"
	}

	return *p
}

func enumValues(a asnType) []string {
	values := slices.Clone(a.Root)

	if ext, ok := a.Ext.([]any); ok {
		for _, v := range ext {
			values = append(values, v.(string))
		}
	}

	return values
}

func normalizedValues(a asnType) []string {
	values := enumValues(a)
	for i, v := range values {
		values[i] = normalizeName(v)
	}

	return values
}

type sourceConst struct {
	name  string
	typ   string
	value int64
}

func packageConsts(t *testing.T) []sourceConst {
	t.Helper()

	var out []sourceConst

	fset := token.NewFileSet()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "per_gen.go" {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}

		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}

			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				if len(vs.Values) != 1 {
					continue
				}

				lit, ok := vs.Values[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.INT {
					continue
				}

				v, err := strconv.ParseInt(lit.Value, 10, 64)
				if err != nil {
					t.Fatal(err)
				}

				typ := ""
				if id, ok := vs.Type.(*ast.Ident); ok {
					typ = id.Name
				}

				out = append(out, sourceConst{name: vs.Names[0].Name, typ: typ, value: v})
			}
		}
	}

	return out
}

func checkConstNames(t *testing.T, group string, consts []sourceConst, names []string, match func(sourceConst) bool) {
	t.Helper()

	prefixes := map[string]bool{}
	found := 0

	for _, sc := range consts {
		if !match(sc) {
			continue
		}

		name := sc.name

		found++

		if sc.value < 0 || sc.value >= int64(len(names)) {
			t.Errorf("%s: %s = %d is outside the %d defined values", group, name, sc.value, len(names))
			continue
		}

		want := normalizeName(names[sc.value])
		got := strings.ToLower(name)

		if !strings.HasSuffix(got, want) {
			t.Errorf("%s: %s = %d, which is %q", group, name, sc.value, names[sc.value])
			continue
		}

		prefixes[got[:len(got)-len(want)]] = true
	}

	if found == 0 {
		t.Errorf("%s: no constants", group)
	}

	if len(prefixes) > 1 {
		t.Errorf("%s: constants use different prefixes %v", group, prefixes)
	}
}

var namedBitConsts = map[string][2]string{
	"GNSSIDBitmap":                {"GNSS-ID-Bitmap", "gnss-ids"},
	"SBASIDs":                     {"SBAS-IDs", "sbas-IDs"},
	"PosModes":                    {"PositioningModes", "posModes"},
	"AccessTypes":                 {"AccessTypes", "accessTypes"},
	"IonoModel":                   {"GNSS-IonosphericModelSupport", "ionoModel"},
	"OTDOAMode":                   {"OTDOA-ProvideCapabilities", "otdoa-Mode"},
	"ECIDMeasSupported":           {"ECID-ProvideCapabilities", "ecid-MeasSupported"},
	"ECIDRequestedMeasurements":   {"ECID-RequestLocationInformation", "requestedMeasurements"},
	"NRECIDMeasSupported":         {"NR-ECID-ProvideCapabilities-r16", "nr-ECID-MeasSupported-r16"},
	"NRECIDRequestedMeasurements": {"NR-ECID-RequestLocationInformation-r16", "requestedMeasurements-r16"},
}

func TestModelMatchesSchema(t *testing.T) {
	raw, err := os.ReadFile("testdata/schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema asnSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	c := &schemaChecker{t: t, schema: schema, visited: map[string]bool{}, enums: map[reflect.Type]asnType{}}
	c.check(schema.Root, reflect.TypeFor[LPPMessage](), goTag{choice: -1}, asnType{Ref: schema.Root})

	consts := packageConsts(t)

	for gt, a := range c.enums {
		typeName := gt.Name()
		checkConstNames(t, typeName, consts, enumValues(a), func(sc sourceConst) bool { return sc.typ == typeName })
	}

	for prefix, loc := range namedBitConsts {
		owner, ok := schema.Types[loc[0]]
		if !ok {
			t.Fatalf("schema has no type %s", loc[0])
		}

		var names []string

		for _, comp := range owner.Comps {
			if comp.Name == loc[1] {
				names = comp.Type.Names
			}
		}

		if len(names) == 0 {
			t.Fatalf("%s.%s has no named bits", loc[0], loc[1])
		}

		checkConstNames(t, prefix, consts, names, func(sc sourceConst) bool {
			return sc.typ == "" && strings.HasPrefix(sc.name, prefix)
		})
	}
}
