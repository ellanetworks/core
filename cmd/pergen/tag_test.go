package main

import (
	"testing"
)

func TestParseTagEmpty(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("")
	if err != nil {
		t.Fatal(err)
	}

	if tag.Name != "" {
		t.Fatalf("Name = %q, want empty", tag.Name)
	}

	if tag.ChoiceIdx != -1 {
		t.Fatalf("ChoiceIdx = %d, want -1", tag.ChoiceIdx)
	}

	if tag.TagMode != TagAuto {
		t.Fatalf("TagMode = %v, want TagAuto", tag.TagMode)
	}
}

func TestParseTagNameOnly(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("INTEGER")
	if err != nil {
		t.Fatal(err)
	}

	if tag.Name != "INTEGER" {
		t.Fatalf("Name = %q", tag.Name)
	}
}

func TestParseTagNameDash(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("-")
	if err != nil {
		t.Fatal(err)
	}

	if tag.Name != "-" {
		t.Fatalf("Name = %q", tag.Name)
	}
}

func TestParseTagRangeFull(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("INTEGER,range:0..255")
	if err != nil {
		t.Fatal(err)
	}

	if tag.Name != "INTEGER" {
		t.Fatalf("Name = %q", tag.Name)
	}

	if !tag.HasRangeLB || !tag.HasRangeUB || tag.RangeLB != 0 || tag.RangeUB != 255 {
		t.Fatalf("Range = lb=%d (has=%v) ub=%d (has=%v)", tag.RangeLB, tag.HasRangeLB, tag.RangeUB, tag.HasRangeUB)
	}

	if tag.RangeExtensible {
		t.Fatal("RangeExtensible = true, want false")
	}
}

func TestParseTagRangeSingleValue(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("range:7")
	if err != nil {
		t.Fatal(err)
	}

	if !tag.HasRangeLB || !tag.HasRangeUB || tag.RangeLB != 7 || tag.RangeUB != 7 {
		t.Fatalf("Range = lb=%d ub=%d", tag.RangeLB, tag.RangeUB)
	}
}

func TestParseTagRangeExtensible(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("range:0..9,...")
	if err != nil {
		t.Fatal(err)
	}

	if tag.RangeLB != 0 || tag.RangeUB != 9 {
		t.Fatalf("Range = %d..%d", tag.RangeLB, tag.RangeUB)
	}

	if !tag.RangeExtensible {
		t.Fatal("RangeExtensible = false, want true")
	}
}

func TestParseTagRangeExtensibleOnly(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("range:...")
	if err != nil {
		t.Fatal(err)
	}

	if !tag.RangeExtensible {
		t.Fatal("RangeExtensible = false, want true")
	}

	if tag.HasRangeLB || tag.HasRangeUB {
		t.Fatal("HasRange bounds should be false")
	}
}

func TestParseTagRangeMINMAX(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("range:MIN..MAX")
	if err != nil {
		t.Fatal(err)
	}

	if tag.HasRangeLB || tag.HasRangeUB {
		t.Fatal("bounds should be absent for MIN..MAX")
	}
}

func TestParseTagSize(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("BIT-STRING,size:0..16")
	if err != nil {
		t.Fatal(err)
	}

	if tag.Name != "BIT-STRING" {
		t.Fatalf("Name = %q", tag.Name)
	}

	if !tag.HasSizeLB || !tag.HasSizeUB || tag.SizeLB != 0 || tag.SizeUB != 16 {
		t.Fatalf("Size = lb=%d ub=%d", tag.SizeLB, tag.SizeUB)
	}
}

func TestParseTagOptional(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("optional")
	if err != nil {
		t.Fatal(err)
	}

	if !tag.Optional {
		t.Fatal("Optional = false")
	}
}

func TestParseTagDefault(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("default:42")
	if err != nil {
		t.Fatal(err)
	}

	if tag.DefaultExpr != "42" {
		t.Fatalf("DefaultExpr = %q", tag.DefaultExpr)
	}
}

func TestParseTagDefaultComplexExpr(t *testing.T) {
	t.Parallel()
	// Parenthesised expression with a comma must not be split.
	tag, err := ParseTag("default:foo(1,2)")
	if err != nil {
		t.Fatal(err)
	}

	if tag.DefaultExpr != "foo(1,2)" {
		t.Fatalf("DefaultExpr = %q", tag.DefaultExpr)
	}
}

func TestParseTagChoice(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("choice:3")
	if err != nil {
		t.Fatal(err)
	}

	if tag.ChoiceIdx != 3 {
		t.Fatalf("ChoiceIdx = %d", tag.ChoiceIdx)
	}
}

func TestParseTagTagModes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		expr string
		mode TagMode
	}{
		{"tag:auto", TagAuto},
		{"tag:implicit", TagImplicit},
		{"tag:explicit", TagExplicit},
	} {
		tag, err := ParseTag(tc.expr)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}

		if tag.TagMode != tc.mode {
			t.Fatalf("%s: TagMode = %v, want %v", tc.expr, tag.TagMode, tc.mode)
		}
	}
}

func TestParseTagExt(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("ext")
	if err != nil {
		t.Fatal(err)
	}

	if !tag.Ext {
		t.Fatal("Ext = false")
	}
}

func TestParseTagExtAdd(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("extadd:5")
	if err != nil {
		t.Fatal(err)
	}

	if tag.ExtAdd != 5 {
		t.Fatalf("ExtAdd = %d", tag.ExtAdd)
	}
}

func TestParseTagCombined(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("INTEGER,range:0..65535,ext")
	if err != nil {
		t.Fatal(err)
	}

	if tag.Name != "INTEGER" {
		t.Fatalf("Name = %q", tag.Name)
	}

	if tag.RangeLB != 0 || tag.RangeUB != 65535 {
		t.Fatalf("Range = %d..%d", tag.RangeLB, tag.RangeUB)
	}

	if !tag.Ext {
		t.Fatal("Ext = false")
	}
}

func TestParseTagEverything(t *testing.T) {
	t.Parallel()

	tag, err := ParseTag("SEQUENCE,optional,default:7,tag:explicit,ext,extadd:3")
	if err != nil {
		t.Fatal(err)
	}

	if tag.Name != "SEQUENCE" {
		t.Fatalf("Name = %q", tag.Name)
	}

	if !tag.Optional {
		t.Fatal("Optional = false")
	}

	if tag.DefaultExpr != "7" {
		t.Fatalf("DefaultExpr = %q", tag.DefaultExpr)
	}

	if tag.TagMode != TagExplicit {
		t.Fatal("TagMode = wrong")
	}

	if !tag.Ext {
		t.Fatal("Ext = false")
	}

	if tag.ExtAdd != 3 {
		t.Fatalf("ExtAdd = %d", tag.ExtAdd)
	}
}

func TestParseTagErrors(t *testing.T) {
	t.Parallel()

	bad := []string{
		"tag:wrong",
		"choice:abc",
		"extadd:xyz",
		"range:abc..5",
		"range:5..xyz",
		"unknownkey:5",
		"default:",
	}
	for _, s := range bad {
		if _, err := ParseTag(s); err == nil {
			t.Errorf("ParseTag(%q): expected error, got nil", s)
		}
	}
}
