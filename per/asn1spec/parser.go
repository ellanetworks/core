// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package asn1spec

import (
	"fmt"
	"strconv"
)

type parser struct {
	toks []token
	i    int
	tok  token
}

type bailout struct {
	err error
}

func newParser(toks []token) *parser {
	return &parser{toks: toks, tok: toks[0]}
}

func (p *parser) next() {
	if p.i < len(p.toks)-1 {
		p.i++
	}

	p.tok = p.toks[p.i]
}

func (p *parser) peek() token {
	if p.i+1 >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}

	return p.toks[p.i+1]
}

func (p *parser) errorf(format string, args ...any) {
	panic(bailout{&Error{Pos: p.tok.pos, Msg: fmt.Sprintf(format, args...)}})
}

func (p *parser) expect(k tokenKind) token {
	t := p.tok
	if t.kind != k {
		p.errorf("expected %q, found %s", k.String(), t)
	}

	p.next()

	return t
}

func (p *parser) got(k tokenKind) bool {
	if p.tok.kind != k {
		return false
	}

	p.next()

	return true
}

func (p *parser) is(word string) bool {
	return p.tok.kind == tokIdent && p.tok.lit == word
}

func (p *parser) gotWord(word string) bool {
	if !p.is(word) {
		return false
	}

	p.next()

	return true
}

func (p *parser) expectWord(word string) {
	if !p.is(word) {
		p.errorf("expected %q, found %s", word, p.tok)
	}

	p.next()
}

func isUpper(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

func catch(err *error) {
	if r := recover(); r != nil {
		b, ok := r.(bailout)
		if !ok {
			panic(r)
		}

		*err = b.err
	}
}

func Parse(filename string, src []byte) (m *Module, err error) {
	toks, err := scan(filename, src)
	if err != nil {
		return nil, err
	}

	defer catch(&err)

	p := newParser(toks)
	m = &Module{Filename: filename, names: map[string]any{}}
	p.parseModule(m)

	if p.tok.kind != tokEOF {
		p.errorf("unexpected %s after END", p.tok)
	}

	for _, o := range m.allObjects() {
		if c := m.Class(o.Class); c != nil {
			if err := resolveObject(c, o); err != nil {
				return nil, err
			}
		}
	}

	return m, nil
}

func (p *parser) parseModule(m *Module) {
	m.Name = p.expect(tokIdent).lit

	if p.tok.kind == tokLBrace {
		m.OID = p.parseOID()
	}

	p.expectWord("DEFINITIONS")

	if p.is("AUTOMATIC") || p.is("EXPLICIT") || p.is("IMPLICIT") {
		m.TagDefault = p.tok.lit
		p.next()
		p.expectWord("TAGS")
	}

	if p.gotWord("EXTENSIBILITY") {
		p.expectWord("IMPLIED")
	}

	p.expect(tokAssign)
	p.expectWord("BEGIN")

	if p.is("IMPORTS") {
		m.Imports = p.parseImports()
	}

	for !p.is("END") {
		if p.tok.kind == tokEOF {
			p.errorf("missing END")
		}

		p.parseAssignment(m)
	}

	p.next()
}

func (p *parser) parseOID() []OIDComponent {
	p.expect(tokLBrace)

	var oid []OIDComponent

	for p.tok.kind != tokRBrace {
		var c OIDComponent

		switch p.tok.kind {
		case tokIdent:
			c.Name = p.tok.lit
			p.next()

			if p.got(tokLParen) {
				c.Number, c.HasNumber = p.parseNumber(), true
				p.expect(tokRParen)
			}
		case tokNumber:
			c.Number, c.HasNumber = p.parseNumber(), true
		default:
			p.errorf("unexpected %s in object identifier", p.tok)
		}

		oid = append(oid, c)
	}

	p.next()

	return oid
}

func (p *parser) parseNumber() int64 {
	t := p.expect(tokNumber)

	n, err := strconv.ParseInt(t.lit, 10, 64)
	if err != nil {
		p.errorf("invalid number %q: %v", t.lit, err)
	}

	return n
}

func (p *parser) parseImports() []Import {
	p.next()

	var imports []Import

	for p.tok.kind != tokSemicolon {
		var imp Import

		for {
			imp.Symbols = append(imp.Symbols, p.expect(tokIdent).lit)

			if p.got(tokLBrace) {
				p.expect(tokRBrace)
			}

			if !p.got(tokComma) {
				break
			}
		}

		p.expectWord("FROM")
		imp.From = p.expect(tokIdent).lit
		imports = append(imports, imp)
	}

	p.next()

	return imports
}

func (m *Module) add(name string, pos Position, x any) {
	if _, dup := m.names[name]; dup {
		panic(bailout{&Error{Pos: pos, Msg: fmt.Sprintf("%s redefined", name)}})
	}

	m.names[name] = x
}

func (p *parser) parseAssignment(m *Module) {
	nameTok := p.expect(tokIdent)
	name, pos := nameTok.lit, nameTok.pos

	switch {
	case p.tok.kind == tokAssign && isUpper(name):
		p.next()

		if p.gotWord("CLASS") {
			c := p.parseClass()
			c.Name, c.Pos = name, pos
			m.Classes = append(m.Classes, c)
			m.add(name, pos, c)

			return
		}

		ta := &TypeAssignment{Name: name, Type: p.parseType(), Pos: pos}
		m.Types = append(m.Types, ta)
		m.add(name, pos, ta)
	case p.tok.kind == tokLBrace && isUpper(name):
		params := p.parseParams()
		p.expect(tokAssign)
		ta := &TypeAssignment{Name: name, Params: params, Type: p.parseType(), Pos: pos}
		m.Types = append(m.Types, ta)
		m.add(name, pos, ta)
	case isUpper(name):
		class := p.expect(tokIdent).lit
		p.expect(tokAssign)
		sa := &ObjectSetAssignment{Name: name, Class: class, Set: p.parseObjectSet(class), Pos: pos}
		m.ObjectSets = append(m.ObjectSets, sa)
		m.add(name, pos, sa)
	default:
		governor := p.parseType()
		p.expect(tokAssign)

		if p.tok.kind == tokLBrace && governor.Kind == Reference && len(governor.Args) == 0 {
			oa := &ObjectAssignment{Name: name, Class: governor.Name, Object: p.captureObject(governor.Name), Pos: pos}
			m.Objects = append(m.Objects, oa)
			m.add(name, pos, oa)

			return
		}

		va := &ValueAssignment{Name: name, Type: governor, Value: p.parseValue(), Pos: pos}
		m.Values = append(m.Values, va)
		m.add(name, pos, va)
	}
}

func (p *parser) parseParams() []Param {
	p.expect(tokLBrace)

	var params []Param

	for {
		var prm Param

		prm.Name = p.expect(tokIdent).lit
		if p.got(tokColon) {
			prm.Governor, prm.Name = prm.Name, p.expect(tokIdent).lit
		}

		params = append(params, prm)

		if !p.got(tokComma) {
			break
		}
	}

	p.expect(tokRBrace)

	return params
}

var characterStringTypes = map[string]bool{
	"BMPString":       true,
	"GeneralString":   true,
	"GraphicString":   true,
	"IA5String":       true,
	"ISO646String":    true,
	"NumericString":   true,
	"PrintableString": true,
	"TeletexString":   true,
	"UniversalString": true,
	"UTF8String":      true,
	"VisibleString":   true,
}

func (p *parser) parseType() *Type {
	t := &Type{Pos: p.tok.pos}

	if p.tok.kind != tokIdent {
		p.errorf("expected type, found %s", p.tok)
	}

	lit := p.tok.lit

	switch {
	case lit == "INTEGER":
		p.next()

		t.Kind = Integer
		if p.tok.kind == tokLBrace {
			t.NamedNumbers = p.parseNamedNumbers()
		}
	case lit == "ENUMERATED":
		p.next()

		t.Kind = Enumerated
		p.parseEnumerated(t)
	case lit == "BIT":
		p.next()
		p.expectWord("STRING")

		t.Kind = BitString
		if p.tok.kind == tokLBrace {
			t.NamedNumbers = p.parseNamedNumbers()
		}
	case lit == "OCTET":
		p.next()
		p.expectWord("STRING")

		t.Kind = OctetString
	case lit == "BOOLEAN":
		p.next()

		t.Kind = Boolean
	case lit == "NULL":
		p.next()

		t.Kind = Null
	case lit == "OBJECT":
		p.next()
		p.expectWord("IDENTIFIER")

		t.Kind = ObjectIdentifier
	case characterStringTypes[lit]:
		p.next()

		t.Kind, t.Name = CharacterString, lit
	case lit == "SEQUENCE":
		p.next()
		p.parseSequence(t)
	case lit == "CHOICE":
		p.next()

		t.Kind = Choice
		p.parseComponents(t, false)
	case isUpper(lit) && !reservedWords[lit]:
		p.next()

		t.Kind, t.Name = Reference, lit

		switch {
		case p.tok.kind == tokDot && p.peek().kind == tokField:
			p.next()

			t.Kind, t.Field = ObjectClassField, p.tok.lit
			p.next()
		case p.tok.kind == tokLBrace:
			t.Args = p.parseActuals()
		}
	default:
		p.errorf("unsupported type %s", p.tok)
	}

	for p.tok.kind == tokLParen {
		p.parseConstraint(t)
	}

	return t
}

var reservedWords = map[string]bool{
	"ABSENT": true, "ALL": true, "ANY": true, "APPLICATION": true, "AUTOMATIC": true, "BEGIN": true,
	"BY": true, "CLASS": true, "COMPONENT": true, "COMPONENTS": true, "CONSTRAINED": true,
	"CONTAINING": true, "DEFAULT": true, "DEFINED": true, "DEFINITIONS": true, "EMBEDDED": true,
	"ENCODED": true, "END": true, "EXCEPT": true, "EXPLICIT": true, "EXPORTS": true, "EXTENSIBILITY": true,
	"EXTERNAL": true, "FALSE": true, "FROM": true, "IDENTIFIER": true, "IMPLICIT": true, "IMPLIED": true,
	"IMPORTS": true, "INCLUDES": true, "INSTANCE": true, "INTERSECTION": true, "MAX": true, "MIN": true,
	"MINUS-INFINITY": true, "OF": true, "OPTIONAL": true, "PATTERN": true, "PDV": true, "PLUS-INFINITY": true,
	"PRESENT": true, "PRIVATE": true, "REAL": true, "SET": true, "SIZE": true, "STRING": true, "SYNTAX": true,
	"TAGS": true, "TRUE": true, "TYPE-IDENTIFIER": true, "UNION": true, "UNIQUE": true, "UNIVERSAL": true,
	"WITH": true,
}

func (p *parser) parseSequence(t *Type) {
	if p.tok.kind == tokLBrace {
		t.Kind = Sequence
		p.parseComponents(t, true)

		return
	}

	t.Kind = SequenceOf

	switch {
	case p.tok.kind == tokLParen && p.peek().kind == tokIdent && p.peek().lit == "SIZE":
		p.next()
		p.next()

		t.Size = p.parseRangeSpec()
		p.expect(tokRParen)
	case p.is("SIZE"):
		p.next()

		t.Size = p.parseRangeSpec()
	}

	p.expectWord("OF")
	t.Elem = p.parseType()
}

func (p *parser) parseNamedNumbers() []NamedNumber {
	p.expect(tokLBrace)

	var nn []NamedNumber

	for {
		n := NamedNumber{Name: p.expect(tokIdent).lit}

		p.expect(tokLParen)
		n.Value = p.parseValue()
		p.expect(tokRParen)

		nn = append(nn, n)

		if !p.got(tokComma) {
			break
		}
	}

	p.expect(tokRBrace)

	return nn
}

func (p *parser) parseEnumerated(t *Type) {
	p.expect(tokLBrace)

	for {
		if p.got(tokEllipsis) {
			if t.Extensible {
				p.errorf("duplicate extension marker")
			}

			t.Extensible = true
		} else {
			name := p.expect(tokIdent)
			if isUpper(name.lit) {
				p.errorf("invalid enumeration item %q", name.lit)
			}

			item := &EnumItem{Name: name.lit, Extension: t.Extensible}

			if p.got(tokLParen) {
				item.Number, item.HasNumber = p.parseSignedNumber(), true
				p.expect(tokRParen)
			}

			t.Enum = append(t.Enum, item)
		}

		if !p.got(tokComma) {
			break
		}
	}

	p.expect(tokRBrace)
}

func (p *parser) parseSignedNumber() int64 {
	if p.got(tokMinus) {
		return -p.parseNumber()
	}

	return p.parseNumber()
}

func (p *parser) parseComponents(t *Type, seq bool) {
	p.expect(tokLBrace)

	ext, group := 0, 0

	for p.tok.kind != tokRBrace {
		switch {
		case p.got(tokEllipsis):
			switch ext {
			case 0:
				t.Extensible, ext = true, 1
			case 1:
				ext = 2
			default:
				p.errorf("too many extension markers")
			}
		case p.tok.kind == tokLVersion:
			if ext != 1 {
				p.errorf("extension addition group outside extension")
			}

			p.next()

			group++

			if p.tok.kind == tokNumber && p.peek().kind == tokColon {
				p.next()
				p.next()
			}

			for {
				c := p.parseComponent(seq)
				c.Extension, c.Group = true, group
				t.Components = append(t.Components, c)

				if !p.got(tokComma) {
					break
				}
			}

			p.expect(tokRVersion)
		default:
			c := p.parseComponent(seq)
			c.Extension = ext == 1
			t.Components = append(t.Components, c)
		}

		if !p.got(tokComma) {
			break
		}
	}

	p.expect(tokRBrace)
}

func (p *parser) parseComponent(seq bool) *Component {
	name := p.expect(tokIdent)
	if isUpper(name.lit) {
		p.errorf("unsupported component %q", name.lit)
	}

	c := &Component{Name: name.lit, Pos: name.pos, Type: p.parseType()}

	if seq {
		switch {
		case p.gotWord("OPTIONAL"):
			c.Optional = true
		case p.gotWord("DEFAULT"):
			v := p.parseValue()
			c.Default = &v
		}
	}

	return c
}

func (p *parser) parseValue() Value {
	switch p.tok.kind {
	case tokNumber:
		return Value{Number: p.expect(tokNumber).lit}
	case tokMinus:
		p.next()

		return Value{Number: "-" + p.expect(tokNumber).lit}
	case tokIdent:
		if isUpper(p.tok.lit) && p.tok.lit != "MIN" && p.tok.lit != "MAX" && p.tok.lit != "TRUE" && p.tok.lit != "FALSE" {
			p.errorf("expected value, found %s", p.tok)
		}

		v := Value{Ref: p.tok.lit}
		p.next()

		return v
	}

	p.errorf("expected value, found %s", p.tok)

	return Value{}
}

func (p *parser) parseConstraint(t *Type) {
	next := p.peek()

	switch {
	case next.kind == tokLBrace:
		if t.Table != nil {
			p.errorf("duplicate table constraint")
		}

		p.next()
		p.next()

		tc := &TableConstraint{Set: p.expect(tokIdent).lit}
		p.expect(tokRBrace)

		if p.got(tokLBrace) {
			p.expect(tokAt)
			tc.At = p.expect(tokIdent).lit
			p.expect(tokRBrace)
		}

		p.expect(tokRParen)

		t.Table = tc
	case next.kind == tokIdent && next.lit == "CONTAINING":
		if t.Containing != nil {
			p.errorf("duplicate contents constraint")
		}

		p.next()
		p.next()

		t.Containing = p.parseType()
		p.expect(tokRParen)
	case next.kind == tokIdent && next.lit == "SIZE":
		if t.Size != nil {
			p.errorf("duplicate size constraint")
		}

		p.next()
		p.next()

		t.Size = p.parseRangeSpec()
		p.expect(tokRParen)
	default:
		if t.Range != nil {
			p.errorf("duplicate value constraint")
		}

		t.Range = p.parseRangeSpec()
	}
}

func (p *parser) parseRangeSpec() *Constraint {
	p.expect(tokLParen)

	c := &Constraint{}

	if p.tok.kind != tokEllipsis {
		c.Root = p.parseRangeUnion()

		if !p.got(tokComma) {
			p.expect(tokRParen)

			return c
		}
	}

	p.expect(tokEllipsis)

	c.Extensible = true

	if p.got(tokComma) {
		c.Ext = p.parseRangeUnion()
	}

	p.expect(tokRParen)

	return c
}

func (p *parser) parseRangeUnion() []Range {
	var rs []Range

	for {
		r := Range{Lower: p.parseValue()}

		r.Upper = r.Lower
		if p.got(tokRange) {
			r.Upper = p.parseValue()
		}

		rs = append(rs, r)

		if !p.got(tokPipe) {
			return rs
		}
	}
}

func (p *parser) parseActuals() []*Actual {
	p.expect(tokLBrace)

	var args []*Actual

	for {
		var a Actual

		switch {
		case p.tok.kind == tokLBrace:
			a.Set = p.parseObjectSet("")
		case p.tok.kind == tokNumber || p.tok.kind == tokMinus || p.tok.kind == tokIdent && !isUpper(p.tok.lit):
			v := p.parseValue()
			a.Value = &v
		default:
			a.Type = p.parseType()
		}

		args = append(args, &a)

		if !p.got(tokComma) {
			break
		}
	}

	p.expect(tokRBrace)

	return args
}

func (p *parser) parseObjectSet(class string) *ObjectSet {
	p.expect(tokLBrace)

	s := &ObjectSet{}

	for p.tok.kind != tokRBrace {
		if p.got(tokEllipsis) {
			if s.Extensible {
				p.errorf("duplicate extension marker")
			}

			s.Extensible = true
		} else {
			var e SetElement

			switch p.tok.kind {
			case tokLBrace:
				e.Object = p.captureObject(class)
			case tokIdent:
				e.Ref = p.tok.lit
				p.next()
			default:
				p.errorf("unexpected %s in object set", p.tok)
			}

			if s.Extensible {
				s.Ext = append(s.Ext, &e)
			} else {
				s.Root = append(s.Root, &e)
			}
		}

		if !p.got(tokComma) && !p.got(tokPipe) {
			break
		}
	}

	p.expect(tokRBrace)

	return s
}

func (p *parser) captureObject(class string) *Object {
	start := p.expect(tokLBrace)
	o := &Object{Class: class, Pos: start.pos}
	depth := 1

	for {
		switch p.tok.kind {
		case tokEOF:
			p.errorf("object not terminated")
		case tokLBrace:
			depth++
		case tokRBrace:
			depth--
			if depth == 0 {
				o.body = append(o.body, token{kind: tokEOF, pos: p.tok.pos})
				p.next()

				return o
			}
		}

		o.body = append(o.body, p.tok)
		p.next()
	}
}

func (p *parser) parseClass() *ClassAssignment {
	c := &ClassAssignment{}

	p.expect(tokLBrace)

	for {
		f := &ClassField{Name: p.expect(tokField).lit}

		if !f.IsTypeField() {
			f.Type = p.parseType()
			f.Unique = p.gotWord("UNIQUE")
		}

		switch {
		case p.gotWord("OPTIONAL"):
			f.Optional = true
		case p.gotWord("DEFAULT"):
			if f.IsTypeField() {
				f.Default = &Setting{Type: p.parseType()}
			} else {
				v := p.parseValue()
				f.Default = &Setting{Value: &v}
			}
		}

		c.Fields = append(c.Fields, f)

		if !p.got(tokComma) {
			break
		}
	}

	p.expect(tokRBrace)

	if p.gotWord("WITH") {
		p.expectWord("SYNTAX")
		p.expect(tokLBrace)
		c.Syntax = p.parseSyntax(false, tokRBrace)
		p.expect(tokRBrace)
	}

	for _, it := range c.Syntax {
		if c.Field(it.Field) == nil {
			p.errorf("syntax refers to undefined field %s", it.Field)
		}
	}

	return c
}

func (p *parser) parseSyntax(optional bool, end tokenKind) []SyntaxItem {
	var (
		items    []SyntaxItem
		keywords []string
	)

	for p.tok.kind != end {
		switch p.tok.kind {
		case tokIdent:
			if !isUpper(p.tok.lit) {
				p.errorf("invalid syntax keyword %q", p.tok.lit)
			}

			keywords = append(keywords, p.tok.lit)
			p.next()
		case tokField:
			items = append(items, SyntaxItem{Keywords: keywords, Field: p.tok.lit, Optional: optional})
			keywords = nil

			p.next()
		case tokLBrack:
			if optional || len(keywords) > 0 {
				p.errorf("unsupported nested optional syntax group")
			}

			p.next()

			group := p.parseSyntax(true, tokRBrack)
			if len(group) != 1 || len(group[0].Keywords) == 0 {
				p.errorf("unsupported optional syntax group")
			}

			items = append(items, group...)

			p.expect(tokRBrack)
		default:
			p.errorf("unexpected %s in WITH SYNTAX", p.tok)
		}
	}

	if len(keywords) > 0 {
		p.errorf("syntax keywords %v not followed by a field", keywords)
	}

	return items
}

func resolveObject(c *ClassAssignment, o *Object) (err error) {
	if o.Settings != nil {
		return nil
	}

	if c.Syntax == nil {
		return &Error{Pos: o.Pos, Msg: fmt.Sprintf("class %s has no WITH SYNTAX", c.Name)}
	}

	defer catch(&err)

	p := newParser(o.body)
	settings := map[string]Setting{}

	for _, it := range c.Syntax {
		if it.Optional && !p.is(it.Keywords[0]) {
			continue
		}

		for _, kw := range it.Keywords {
			p.expectWord(kw)
		}

		f := c.Field(it.Field)
		if f.IsTypeField() {
			settings[f.Name] = Setting{Type: p.parseType()}
		} else {
			v := p.parseValue()
			settings[f.Name] = Setting{Value: &v}
		}
	}

	if p.tok.kind != tokEOF {
		p.errorf("unexpected %s in object of class %s", p.tok, c.Name)
	}

	o.Class, o.Settings = c.Name, settings

	return nil
}

func (m *Module) allObjects() []*Object {
	var objs []*Object

	for _, oa := range m.Objects {
		objs = append(objs, oa.Object)
	}

	for _, sa := range m.ObjectSets {
		for _, e := range append(append([]*SetElement{}, sa.Set.Root...), sa.Set.Ext...) {
			if e.Object != nil {
				objs = append(objs, e.Object)
			}
		}
	}

	return objs
}
