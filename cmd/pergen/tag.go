// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// FieldTag holds the parsed `per:"..."` tag for a single struct field or named
// type declaration.
type FieldTag struct {
	// Raw is the original tag text (without the leading "per:").
	Raw string

	// Name is the optional ASN.1 type name (the first comma-separated token),
	// e.g. "INTEGER", "BIT-STRING", "SEQUENCE". Empty means infer from the Go
	// type. Reserved value "-" means "no name, use Go type".
	Name string

	// Range encodes INTEGER (lb..ub) constraints. HasLB/HasUB indicate which
	// bound is present (MIN/MAX absent them).
	RangeLB, RangeUB       int64
	HasRangeLB, HasRangeUB bool
	RangeExtensible        bool

	// Size encodes SIZE (lb..ub) constraints for BIT/OCTET STRING and strings.
	SizeLB, SizeUB       int64
	HasSizeLB, HasSizeUB bool
	SizeExtensible       bool

	// Optional marks the field as OPTIONAL in a SEQUENCE.
	Optional bool
	// DefaultExpr is the Go expression for a DEFAULT value (parsed, not
	// evaluated, by pergen — it's emitted verbatim into the generated code).
	DefaultExpr string

	// ChoiceIdx is the index of this alternative in a CHOICE. -1 = not part of
	// a CHOICE.
	ChoiceIdx int

	// TagMode selects AUTOMATIC/IMPLICIT/EXPLICIT tagging for the field.
	TagMode TagMode

	// Ext marks a SEQUENCE/CHOICE/SET type as extensible ("...").
	Ext bool
	// ExtAdd is the count of extension additions ([[...]]).
	ExtAdd int
	// ExtSeq marks the parent type as extensible ("...") without any actual
	// extension additions. It is placed on a placeholder field (typically
	// named "_") that pergen skips for encoding. This allows modelling ASN.1
	// types like `SEQUENCE { ..., ... }` where the "..." is present but no
	// extension additions exist yet.
	ExtSeq bool
}

// TagMode is the tagging mode for a field.
type TagMode uint8

const (
	TagAuto TagMode = iota // AUTOMATIC (default): inherit module tagging
	TagImplicit
	TagExplicit
)

// ParseTag parses a `per:"..."` tag value (the part inside the quotes, without
// the key "per:"). It returns an error on malformed syntax.
//
// Grammar (whitespace tolerated, commas separate tokens):
//
//	tag        := name? ("," option)*
//	name       := "-" | identifier
//	option     := "optional"
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
	// Merge "..." extensible-suffix parts back into the preceding range/size
	// option (handles "range:0..9,..." where the ellipsis belongs to range).
	parts = mergeEllipsis(parts)
	if len(parts) == 0 {
		return t, nil
	}
	// First part may be a name or an option. A name is present when it is not
	// of the form "key:value" or a bare keyword (optional/ext).
	first := strings.TrimSpace(parts[0])
	if first != "" && !looksLikeOption(first) {
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

// looksLikeOption reports whether token is a known option keyword or key:value
// pair (vs. a bare ASN.1 type name).
func looksLikeOption(token string) bool {
	if token == "optional" || token == "ext" || token == "extseq" {
		return true
	}

	if i := strings.IndexByte(token, ':'); i > 0 {
		key := strings.TrimSpace(token[:i])
		switch key {
		case "range", "size", "default", "choice", "tag", "extadd":
			return true
		}
		// Unknown "key:value" — treat as option so the parser reports an error
		// rather than silently accepting it as a type name.
		return true
	}

	return false
}

func parseOption(t *FieldTag, opt string) error {
	switch opt {
	case "optional":
		t.Optional = true
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

// splitKV splits "key:value" into key and value (value may contain colons; the
// split is on the first colon). Returns ok=false if there's no colon.
func splitKV(opt string) (key, value string, ok bool) {
	before, after, ok := strings.Cut(opt, ":")
	if !ok {
		return "", "", false
	}

	return strings.TrimSpace(before), strings.TrimSpace(after), true
}

// parseRange parses a range expression ("lb..ub", "lb", "MIN..MAX", "...").
// extensible marker ",..." appended to a range sets *ext=true.
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
	// Single value: lb == ub.
	if err := parseBound(v, lb, hasLB); err != nil {
		return err
	}

	*ub = *lb
	*hasUB = true

	return nil
}

// parseBound parses a single bound: "MIN", "MAX", or a signed integer.
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

// splitTopLevelCommas splits s on commas that are not inside parentheses or
// quotes. This allows `default:foo(1,2)` to be passed as one option.
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

// mergeEllipsis merges a trailing "..." option (the extensible-suffix form
// "range:0..9,...") into the preceding range/size option. PER range extensible
// suffix is written as "range:lo..hi,...", and splitTopLevelCommas separates
// the "..."; this re-attaches it as a ",..." suffix.
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
