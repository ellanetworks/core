// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// FieldTag holds the parsed `per:"..."` tag of a struct field.
type FieldTag struct {
	// Raw is the tag text without the leading "per:".
	Raw string

	// Name is the ASN.1 type name (first comma-separated token). Empty or "-"
	// means infer from the Go type.
	Name string

	// MIN/MAX bounds leave HasRangeLB/HasRangeUB false.
	RangeLB, RangeUB       int64
	HasRangeLB, HasRangeUB bool
	RangeExtensible        bool

	// SIZE constraint for BIT/OCTET STRING, strings and SEQUENCE OF.
	SizeLB, SizeUB       int64
	HasSizeLB, HasSizeUB bool
	SizeExtensible       bool

	Optional bool
	// Skip marks an OPTIONAL field the type does not model: encoded as
	// always-absent, read and discarded on decode.
	Skip bool
	// DefaultExpr is emitted verbatim into the generated code, not evaluated.
	DefaultExpr string

	// ChoiceIdx is -1 when the field is not a CHOICE alternative.
	ChoiceIdx int

	TagMode TagMode

	// Ext marks the field as an extension addition ("[[...]]").
	Ext bool
	// ExtAdd is the count of extension additions ([[...]]).
	ExtAdd int
	// ExtSeq marks the parent type extensible ("...") with no extension
	// additions; it sits on a placeholder field that carries no encoding.
	ExtSeq bool
}

type TagMode uint8

const (
	TagAuto TagMode = iota // AUTOMATIC (default): inherit module tagging
	TagImplicit
	TagExplicit
)

// ParseTag parses a `per:"..."` tag value, the part inside the quotes.
//
// Grammar (whitespace tolerated, commas separate tokens):
//
//	tag        := name? ("," option)*
//	name       := "-" | identifier
//	option     := "optional"
//	            | "skip"
//	            | "ext"
//	            | "extadd:" int
//	            | "range:" range
//	            | "size:" range
//	            | "default:" goexpr
//	            | "choice:" int
//	            | "tag:" tagmode
//	range      := [bound] ".." [bound] [",..."]   -- at least one bound
//	            | bound [",..."]                    -- single value
//	            | "..."
//	bound      := "MIN" | "MAX" | int
//	tagmode    := "auto" | "implicit" | "explicit"
func ParseTag(s string) (FieldTag, error) {
	t := FieldTag{ChoiceIdx: -1, TagMode: TagAuto}
	t.Raw = s
	parts := splitTopLevelCommas(s)
	parts = mergeEllipsis(parts)

	if len(parts) == 0 {
		return t, nil
	}

	first := strings.TrimSpace(parts[0])
	if first != "" && !looksLikeOption(first) {
		if !knownTypeName(first) {
			return t, fmt.Errorf("per: unknown ASN.1 type name %q", first)
		}

		t.Name = first
		parts = parts[1:]
	}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if err := parseOption(&t, p); err != nil {
			return t, fmt.Errorf("per: bad option %q: %w", p, err)
		}
	}

	return t, nil
}

// asn1TypeNames are the names legal in the first tag position. A name outside
// this set is rejected, since falling back to Go-type inference would let a
// typo silently change the wire format.
var asn1TypeNames = map[string]bool{
	"-":               true,
	"BIT-STRING":      true,
	"BMPString":       true,
	"BOOLEAN":         true,
	"ENUMERATED":      true,
	"IA5String":       true,
	"INTEGER":         true,
	"NULL":            true,
	"NumericString":   true,
	"OCTET-STRING":    true,
	"PrintableString": true,
	"REAL":            true,
	"SEQUENCE":        true,
	"SEQUENCE-OF":     true,
	"SET":             true,
	"UTF8String":      true,
	"UniversalString": true,
	"VisibleString":   true,
}

func knownTypeName(name string) bool { return asn1TypeNames[name] }

// looksLikeOption distinguishes an option token from a bare ASN.1 type name.
func looksLikeOption(token string) bool {
	if token == "optional" || token == "ext" || token == "extseq" || token == "skip" {
		return true
	}

	if i := strings.IndexByte(token, ':'); i > 0 {
		key := strings.TrimSpace(token[:i])
		switch key {
		case "range", "size", "default", "choice", "tag", "extadd":
			return true
		}
		// Unknown "key:value" is claimed here so parseOption reports an error
		// instead of the token being accepted as a type name.
		return true
	}

	return false
}

func parseOption(t *FieldTag, opt string) error {
	switch opt {
	case "optional":
		t.Optional = true
		return nil
	case "skip":
		t.Skip = true
		return nil
	case "ext":
		t.Ext = true
		return nil
	case "extseq":
		t.ExtSeq = true
		return nil
	}

	key, value, ok := splitKV(opt)
	if !ok {
		return errors.New("unknown option")
	}

	switch key {
	case "range":
		return parseRange(value, &t.RangeLB, &t.RangeUB, &t.HasRangeLB, &t.HasRangeUB, &t.RangeExtensible)
	case "size":
		return parseRange(value, &t.SizeLB, &t.SizeUB, &t.HasSizeLB, &t.HasSizeUB, &t.SizeExtensible)
	case "default":
		if value == "" {
			return errors.New("default: empty value")
		}

		t.DefaultExpr = value

		return nil
	case "choice":
		idx, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("choice: %w", err)
		}

		t.ChoiceIdx = idx

		return nil
	case "tag":
		switch value {
		case "auto":
			t.TagMode = TagAuto
		case "implicit":
			t.TagMode = TagImplicit
		case "explicit":
			t.TagMode = TagExplicit
		default:
			return fmt.Errorf("tag: unknown mode %q", value)
		}

		return nil
	case "extadd":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("extadd: %w", err)
		}

		t.ExtAdd = n

		return nil
	default:
		return fmt.Errorf("unknown key %q", key)
	}
}

// splitKV splits on the first colon, so a value may itself contain colons.
func splitKV(opt string) (key, value string, ok bool) {
	before, after, ok := strings.Cut(opt, ":")
	if !ok {
		return "", "", false
	}

	return strings.TrimSpace(before), strings.TrimSpace(after), true
}

// parseRange accepts "lb..ub", "lb", "MIN..MAX" and "...", with an optional
// ",..." extensibility suffix.
func parseRange(
	v string,
	lb, ub *int64,
	hasLB, hasUB *bool,
	ext *bool,
) error {
	v = strings.TrimSpace(v)
	if strings.HasSuffix(v, ",...") {
		*ext = true
		v = strings.TrimSpace(v[:len(v)-len(",...")])
	}

	if v == "..." {
		*ext = true
		return nil
	}

	if before, after, ok := strings.Cut(v, ".."); ok {
		lo := strings.TrimSpace(before)
		hi := strings.TrimSpace(after)

		if err := parseBound(lo, lb, hasLB); err != nil {
			return err
		}

		return parseBound(hi, ub, hasUB)
	}

	if err := parseBound(v, lb, hasLB); err != nil {
		return err
	}

	*ub = *lb
	*hasUB = true

	return nil
}

// parseBound accepts "MIN", "MAX" or a signed integer; MIN and MAX leave *has
// false, so an absent bound and an unbounded one are indistinguishable.
func parseBound(s string, out *int64, has *bool) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	if s == "MIN" {
		*has = false
		return nil
	}

	if s == "MAX" {
		*has = false
		return nil
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("bound %q: %w", s, err)
	}

	*out = n
	*has = true

	return nil
}

// splitTopLevelCommas ignores commas nested in parentheses, so
// `default:foo(1,2)` stays one option.
func splitTopLevelCommas(s string) []string {
	var parts []string

	depth := 0
	last := 0

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[last:i])
				last = i + 1
			}
		}
	}

	parts = append(parts, s[last:])

	return parts
}

// mergeEllipsis re-attaches a "..." part that splitTopLevelCommas detached from
// a preceding "range:lo..hi,..." or "size:lo..hi,..." option.
func mergeEllipsis(parts []string) []string {
	if len(parts) < 2 {
		return parts
	}

	merged := make([]string, 0, len(parts))
	for i := range parts {
		p := strings.TrimSpace(parts[i])
		if p == "..." && len(merged) > 0 {
			prev := merged[len(merged)-1]
			if strings.HasPrefix(prev, "range:") || strings.HasPrefix(prev, "size:") {
				merged[len(merged)-1] = prev + ",..."
				continue
			}
		}

		merged = append(merged, parts[i])
	}

	return merged
}
