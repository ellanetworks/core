// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package asn1spec

import (
	"fmt"
	"strconv"
	"strings"
)

type Kind int

const (
	Reference Kind = iota
	Integer
	Enumerated
	BitString
	OctetString
	Boolean
	Null
	ObjectIdentifier
	CharacterString
	Sequence
	SequenceOf
	Choice
	ObjectClassField
)

var kindNames = [...]string{
	Reference:        "reference",
	Integer:          "INTEGER",
	Enumerated:       "ENUMERATED",
	BitString:        "BIT STRING",
	OctetString:      "OCTET STRING",
	Boolean:          "BOOLEAN",
	Null:             "NULL",
	ObjectIdentifier: "OBJECT IDENTIFIER",
	CharacterString:  "character string",
	Sequence:         "SEQUENCE",
	SequenceOf:       "SEQUENCE OF",
	Choice:           "CHOICE",
	ObjectClassField: "class field",
}

func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}

	return "Kind(" + strconv.Itoa(int(k)) + ")"
}

type Value struct {
	Ref    string
	Number string
}

func (v Value) Int64() (int64, error) {
	if v.Ref != "" {
		return 0, fmt.Errorf("asn1spec: %s is a reference", v.Ref)
	}

	return strconv.ParseInt(v.Number, 10, 64)
}

func (v Value) IsRef() bool { return v.Ref != "" }

func (v Value) String() string {
	if v.Ref != "" {
		return v.Ref
	}

	return v.Number
}

type Range struct {
	Lower Value
	Upper Value
}

func (r Range) String() string {
	if r.Lower == r.Upper {
		return r.Lower.String()
	}

	return r.Lower.String() + ".." + r.Upper.String()
}

type Constraint struct {
	Root       []Range
	Extensible bool
	Ext        []Range
}

func (c *Constraint) String() string {
	if c == nil {
		return ""
	}

	parts := make([]string, 0, len(c.Root))
	for _, r := range c.Root {
		parts = append(parts, r.String())
	}

	s := strings.Join(parts, "|")
	if c.Extensible {
		s += ", ..."
	}

	for _, r := range c.Ext {
		s += ", " + r.String()
	}

	return s
}

type TableConstraint struct {
	Set string
	At  string
}

type Actual struct {
	Value *Value
	Set   *ObjectSet
	Type  *Type
}

type Component struct {
	Name      string
	Type      *Type
	Optional  bool
	Default   *Value
	Extension bool
	Group     int
	Pos       Position
}

type EnumItem struct {
	Name      string
	Number    int64
	HasNumber bool
	Extension bool
}

type NamedNumber struct {
	Name  string
	Value Value
}

type Type struct {
	Kind         Kind
	Name         string
	Field        string
	Args         []*Actual
	Components   []*Component
	Extensible   bool
	Elem         *Type
	Enum         []*EnumItem
	NamedNumbers []NamedNumber
	Range        *Constraint
	Size         *Constraint
	Containing   *Type
	Table        *TableConstraint
	Pos          Position
}

func (t *Type) Component(name string) *Component {
	for _, c := range t.Components {
		if c.Name == name {
			return c
		}
	}

	return nil
}

func (t *Type) String() string {
	if t == nil {
		return "<nil>"
	}

	var b strings.Builder

	switch t.Kind {
	case Reference:
		b.WriteString(t.Name)

		if len(t.Args) > 0 {
			args := make([]string, 0, len(t.Args))
			for _, a := range t.Args {
				args = append(args, a.String())
			}

			b.WriteString(" {" + strings.Join(args, ", ") + "}")
		}
	case CharacterString:
		b.WriteString(t.Name)
	case ObjectClassField:
		b.WriteString(t.Name + "." + t.Field)
	case SequenceOf:
		b.WriteString("SEQUENCE")

		if t.Size != nil {
			b.WriteString(" (SIZE(" + t.Size.String() + "))")
		}

		b.WriteString(" OF " + t.Elem.String())

		return b.String()
	default:
		b.WriteString(t.Kind.String())
	}

	if t.Containing != nil {
		b.WriteString(" (CONTAINING " + t.Containing.String() + ")")
	}

	if t.Range != nil {
		b.WriteString(" (" + t.Range.String() + ")")
	}

	if t.Size != nil {
		b.WriteString(" (SIZE(" + t.Size.String() + "))")
	}

	return b.String()
}

func (a *Actual) String() string {
	switch {
	case a.Value != nil:
		return a.Value.String()
	case a.Type != nil:
		return a.Type.String()
	case a.Set != nil:
		return "{" + a.Set.String() + "}"
	}

	return ""
}

type Param struct {
	Governor string
	Name     string
}

type TypeAssignment struct {
	Name   string
	Params []Param
	Type   *Type
	Pos    Position
}

type ValueAssignment struct {
	Name  string
	Type  *Type
	Value Value
	Pos   Position
}

type ClassField struct {
	Name     string
	Type     *Type
	Unique   bool
	Optional bool
	Default  *Setting
}

func (f *ClassField) IsTypeField() bool {
	return len(f.Name) > 1 && f.Name[1] >= 'A' && f.Name[1] <= 'Z'
}

type SyntaxItem struct {
	Keywords []string
	Field    string
	Optional bool
}

type ClassAssignment struct {
	Name   string
	Fields []*ClassField
	Syntax []SyntaxItem
	Pos    Position
}

func (c *ClassAssignment) Field(name string) *ClassField {
	for _, f := range c.Fields {
		if f.Name == name {
			return f
		}
	}

	return nil
}

type Setting struct {
	Type  *Type
	Value *Value
}

type Object struct {
	Class    string
	Settings map[string]Setting
	Pos      Position
	body     []token
}

func (o *Object) Value(field string) (Value, bool) {
	s, ok := o.Settings[field]
	if !ok || s.Value == nil {
		return Value{}, false
	}

	return *s.Value, true
}

func (o *Object) Type(field string) *Type {
	return o.Settings[field].Type
}

type SetElement struct {
	Ref    string
	Object *Object
}

type ObjectSet struct {
	Root       []*SetElement
	Extensible bool
	Ext        []*SetElement
}

func (s *ObjectSet) String() string {
	elems := make([]string, 0, len(s.Root)+len(s.Ext)+1)
	for _, e := range s.Root {
		elems = append(elems, e.String())
	}

	if s.Extensible {
		elems = append(elems, "...")
	}

	for _, e := range s.Ext {
		elems = append(elems, e.String())
	}

	return strings.Join(elems, ", ")
}

func (e *SetElement) String() string {
	if e.Object != nil {
		return fmt.Sprintf("{object %s}", e.Object.Pos)
	}

	return e.Ref
}

type ObjectAssignment struct {
	Name   string
	Class  string
	Object *Object
	Pos    Position
}

type ObjectSetAssignment struct {
	Name  string
	Class string
	Set   *ObjectSet
	Pos   Position
}

type OIDComponent struct {
	Name      string
	Number    int64
	HasNumber bool
}

type Import struct {
	Symbols []string
	From    string
}

type Module struct {
	Name       string
	OID        []OIDComponent
	TagDefault string
	Imports    []Import
	Types      []*TypeAssignment
	Values     []*ValueAssignment
	Classes    []*ClassAssignment
	Objects    []*ObjectAssignment
	ObjectSets []*ObjectSetAssignment
	Filename   string
	names      map[string]any
}

func (m *Module) lookup(name string) any {
	return m.names[name]
}

func (m *Module) Type(name string) *TypeAssignment {
	t, _ := m.lookup(name).(*TypeAssignment)
	return t
}

func (m *Module) Value(name string) *ValueAssignment {
	v, _ := m.lookup(name).(*ValueAssignment)
	return v
}

func (m *Module) Class(name string) *ClassAssignment {
	c, _ := m.lookup(name).(*ClassAssignment)
	return c
}

func (m *Module) Object(name string) *ObjectAssignment {
	o, _ := m.lookup(name).(*ObjectAssignment)
	return o
}

func (m *Module) ObjectSet(name string) *ObjectSetAssignment {
	s, _ := m.lookup(name).(*ObjectSetAssignment)
	return s
}

func (m *Module) Defines(name string) bool {
	_, ok := m.names[name]
	return ok
}
