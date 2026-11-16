// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package asn1spec

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
)

type Spec struct {
	Modules []*Module
	byName  map[string]*Module
}

func ParseFS(fsys fs.FS, patterns ...string) (*Spec, error) {
	var files []string

	for _, pattern := range patterns {
		list, err := fs.Glob(fsys, pattern)
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return nil, fmt.Errorf("asn1spec: pattern matches no files: %#q", pattern)
		}

		files = append(files, list...)
	}

	sort.Strings(files)

	mods := make([]*Module, 0, len(files))

	for _, name := range files {
		src, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}

		m, err := Parse(path.Base(name), src)
		if err != nil {
			return nil, err
		}

		mods = append(mods, m)
	}

	return NewSpec(mods...)
}

func NewSpec(mods ...*Module) (*Spec, error) {
	s := &Spec{Modules: mods, byName: map[string]*Module{}}

	for _, m := range mods {
		if _, dup := s.byName[m.Name]; dup {
			return nil, fmt.Errorf("asn1spec: module %s defined twice", m.Name)
		}

		s.byName[m.Name] = m
	}

	for _, m := range mods {
		for _, imp := range m.Imports {
			from := s.byName[imp.From]
			if from == nil {
				return nil, fmt.Errorf("asn1spec: module %s imports from unknown module %s", m.Name, imp.From)
			}

			for _, sym := range imp.Symbols {
				if s.lookup(from, sym) == nil {
					return nil, fmt.Errorf("asn1spec: module %s imports %s which %s does not define", m.Name, sym, imp.From)
				}
			}
		}
	}

	for _, m := range mods {
		for _, o := range m.allObjects() {
			c, _ := s.lookup(m, o.Class).(*ClassAssignment)
			if c == nil {
				return nil, &Error{Pos: o.Pos, Msg: fmt.Sprintf("unknown class %s", o.Class)}
			}

			if err := resolveObject(c, o); err != nil {
				return nil, err
			}
		}
	}

	var errs []error

	for _, m := range mods {
		errs = append(errs, s.check(m)...)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return s, nil
}

func (s *Spec) Resolve(m *Module, name string) (*Module, any) {
	return s.resolve(m, name, 0)
}

func (s *Spec) lookup(m *Module, name string) any {
	_, x := s.resolve(m, name, 0)
	return x
}

func (s *Spec) resolve(m *Module, name string, depth int) (*Module, any) {
	if x := m.lookup(name); x != nil {
		return m, x
	}

	if depth > len(s.Modules) {
		return nil, nil
	}

	for _, imp := range m.Imports {
		for _, sym := range imp.Symbols {
			if sym == name {
				if from := s.byName[imp.From]; from != nil {
					return s.resolve(from, name, depth+1)
				}
			}
		}
	}

	return nil, nil
}

func (s *Spec) Module(name string) *Module {
	return s.byName[name]
}

func (s *Spec) find(name string) any {
	for _, m := range s.Modules {
		if x := m.lookup(name); x != nil {
			return x
		}
	}

	return nil
}

func (s *Spec) Type(name string) *TypeAssignment {
	t, _ := s.find(name).(*TypeAssignment)
	return t
}

func (s *Spec) Value(name string) *ValueAssignment {
	v, _ := s.find(name).(*ValueAssignment)
	return v
}

func (s *Spec) Class(name string) *ClassAssignment {
	c, _ := s.find(name).(*ClassAssignment)
	return c
}

func (s *Spec) Object(name string) *ObjectAssignment {
	o, _ := s.find(name).(*ObjectAssignment)
	return o
}

func (s *Spec) ObjectSet(name string) *ObjectSetAssignment {
	o, _ := s.find(name).(*ObjectSetAssignment)
	return o
}

func (s *Spec) Int(v Value) (int64, error) {
	for range len(s.Modules) + 64 {
		if !v.IsRef() {
			return v.Int64()
		}

		va := s.Value(v.Ref)
		if va == nil {
			return 0, fmt.Errorf("asn1spec: undefined value %s", v.Ref)
		}

		v = va.Value
	}

	return 0, fmt.Errorf("asn1spec: value reference cycle at %s", v.Ref)
}

func (s *Spec) Underlying(t *Type) (*Type, error) {
	seen := map[string]bool{}

	for t.Kind == Reference && len(t.Args) == 0 {
		if seen[t.Name] {
			return nil, fmt.Errorf("asn1spec: type reference cycle at %s", t.Name)
		}

		seen[t.Name] = true

		ta := s.Type(t.Name)
		if ta == nil {
			return nil, fmt.Errorf("asn1spec: undefined type %s", t.Name)
		}

		if len(ta.Params) > 0 {
			return t, nil
		}

		t = ta.Type
	}

	return t, nil
}

func (s *Spec) Objects(set *ObjectSet) ([]*Object, error) {
	var out []*Object

	err := s.collect(set, &out, map[string]bool{})

	return out, err
}

func (s *Spec) collect(set *ObjectSet, out *[]*Object, seen map[string]bool) error {
	for _, e := range append(append([]*SetElement{}, set.Root...), set.Ext...) {
		if e.Object != nil {
			*out = append(*out, e.Object)
			continue
		}

		if seen[e.Ref] {
			return fmt.Errorf("asn1spec: object set cycle at %s", e.Ref)
		}

		seen[e.Ref] = true

		switch x := s.find(e.Ref).(type) {
		case *ObjectAssignment:
			*out = append(*out, x.Object)
		case *ObjectSetAssignment:
			if err := s.collect(x.Set, out, seen); err != nil {
				return err
			}
		default:
			return fmt.Errorf("asn1spec: undefined object or object set %s", e.Ref)
		}

		delete(seen, e.Ref)
	}

	return nil
}

type checker struct {
	s      *Spec
	m      *Module
	params map[string]bool
	errs   []error
}

func (s *Spec) check(m *Module) []error {
	c := &checker{s: s, m: m}

	for _, ta := range m.Types {
		c.params = map[string]bool{}
		for _, prm := range ta.Params {
			c.params[prm.Name] = true
			if prm.Governor != "" && prm.Governor != "INTEGER" {
				c.class(ta.Pos, prm.Governor)
			}
		}

		c.typ(ta.Type)
	}

	c.params = nil

	for _, va := range m.Values {
		c.typ(va.Type)
		c.value(va.Pos, va.Type, va.Value)
	}

	for _, ca := range m.Classes {
		for _, f := range ca.Fields {
			if f.Type != nil {
				c.typ(f.Type)
			}

			if f.Default != nil && f.Default.Type != nil {
				c.typ(f.Default.Type)
			}

			if f.Default != nil && f.Default.Value != nil {
				c.value(ca.Pos, f.Type, *f.Default.Value)
			}
		}
	}

	for _, oa := range m.Objects {
		c.class(oa.Pos, oa.Class)
		c.object(oa.Object)
	}

	for _, sa := range m.ObjectSets {
		c.class(sa.Pos, sa.Class)
		c.set(sa.Pos, sa.Set)
	}

	return c.errs
}

func (c *checker) errorf(pos Position, format string, args ...any) {
	c.errs = append(c.errs, &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

func (c *checker) class(pos Position, name string) {
	if _, ok := c.s.lookup(c.m, name).(*ClassAssignment); !ok {
		c.errorf(pos, "undefined class %s", name)
	}
}

func (c *checker) typ(t *Type) {
	if t == nil {
		return
	}

	switch t.Kind {
	case Reference:
		if c.params[t.Name] {
			break
		}

		ta, ok := c.s.lookup(c.m, t.Name).(*TypeAssignment)
		if !ok {
			c.errorf(t.Pos, "undefined type %s", t.Name)
			break
		}

		if len(ta.Params) != len(t.Args) {
			c.errorf(t.Pos, "%s takes %d parameters, got %d", t.Name, len(ta.Params), len(t.Args))
		}

		for _, a := range t.Args {
			switch {
			case a.Value != nil:
				c.value(t.Pos, nil, *a.Value)
			case a.Type != nil:
				c.typ(a.Type)
			case a.Set != nil:
				c.set(t.Pos, a.Set)
			}
		}
	case ObjectClassField:
		ca, ok := c.s.lookup(c.m, t.Name).(*ClassAssignment)
		if !ok {
			c.errorf(t.Pos, "undefined class %s", t.Name)
		} else if ca.Field(t.Field) == nil {
			c.errorf(t.Pos, "class %s has no field %s", t.Name, t.Field)
		}
	case Sequence, Choice:
		for _, comp := range t.Components {
			c.typ(comp.Type)
		}
	case SequenceOf:
		c.typ(t.Elem)
	case Enumerated:
		seen := map[string]bool{}
		for _, e := range t.Enum {
			if seen[e.Name] {
				c.errorf(t.Pos, "duplicate enumeration item %s", e.Name)
			}

			seen[e.Name] = true
		}
	}

	c.constraint(t.Pos, t.Range)
	c.constraint(t.Pos, t.Size)
	c.typ(t.Containing)

	if t.Table != nil && !c.params[t.Table.Set] {
		if _, ok := c.s.lookup(c.m, t.Table.Set).(*ObjectSetAssignment); !ok {
			c.errorf(t.Pos, "undefined object set %s", t.Table.Set)
		}
	}
}

func (c *checker) constraint(pos Position, cs *Constraint) {
	if cs == nil {
		return
	}

	for _, r := range append(append([]Range{}, cs.Root...), cs.Ext...) {
		c.value(pos, nil, r.Lower)
		c.value(pos, nil, r.Upper)
	}
}

func (c *checker) value(pos Position, governor *Type, v Value) {
	c.valueOf(pos, c, governor, v)
}

func (c *checker) valueOf(pos Position, scope *checker, governor *Type, v Value) {
	if !v.IsRef() || c.params[v.Ref] || v.Ref == "MIN" || v.Ref == "MAX" || v.Ref == "TRUE" || v.Ref == "FALSE" {
		return
	}

	if governor != nil {
		if u, err := scope.underlying(governor); err == nil && u.Kind == Enumerated {
			for _, e := range u.Enum {
				if e.Name == v.Ref {
					return
				}
			}

			c.errorf(pos, "%s is not a literal of %s", v.Ref, governor)

			return
		}
	}

	if _, ok := c.s.lookup(c.m, v.Ref).(*ValueAssignment); !ok {
		c.errorf(pos, "undefined value %s", v.Ref)
	}
}

func (c *checker) underlying(t *Type) (*Type, error) {
	for range 64 {
		if t.Kind != Reference || len(t.Args) > 0 {
			return t, nil
		}

		ta, ok := c.s.lookup(c.m, t.Name).(*TypeAssignment)
		if !ok {
			return nil, fmt.Errorf("undefined type %s", t.Name)
		}

		t = ta.Type
	}

	return nil, fmt.Errorf("type reference too deep at %s", t.Name)
}

func (c *checker) set(pos Position, set *ObjectSet) {
	for _, e := range append(append([]*SetElement{}, set.Root...), set.Ext...) {
		if e.Object != nil {
			c.object(e.Object)
			continue
		}

		if c.params[e.Ref] {
			continue
		}

		switch c.s.lookup(c.m, e.Ref).(type) {
		case *ObjectAssignment, *ObjectSetAssignment:
		default:
			c.errorf(pos, "undefined object or object set %s", e.Ref)
		}
	}
}

func (c *checker) object(o *Object) {
	cm, x := c.s.resolve(c.m, o.Class, 0)

	ca, ok := x.(*ClassAssignment)
	if !ok {
		return
	}

	fc := &checker{s: c.s, m: cm}

	for name, st := range o.Settings {
		f := ca.Field(name)

		switch {
		case st.Type != nil:
			c.typ(st.Type)
		case st.Value != nil:
			c.valueOf(o.Pos, fc, f.Type, *st.Value)
		}
	}

	for _, f := range ca.Fields {
		if _, ok := o.Settings[f.Name]; !ok && !f.Optional && f.Default == nil {
			c.errorf(o.Pos, "object of class %s lacks mandatory field %s", ca.Name, f.Name)
		}
	}
}
