// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"bufio"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const goldenPayloadsPath = "testdata/golden_commands/payloads.txt"

type payloadShape struct {
	schema int
	fields []string
}

var (
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

func describePayload(t reflect.Type) []string {
	var out []string

	describeType(t, "", map[reflect.Type]bool{}, &out)

	return out
}

func describeType(t reflect.Type, path string, visiting map[reflect.Type]bool, out *[]string) {
	if t.Kind() != reflect.Pointer && (t.Implements(jsonMarshalerType) || reflect.PointerTo(t).Implements(jsonMarshalerType) ||
		t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType)) {
		*out = append(*out, fmt.Sprintf("%s %s", pathOrRoot(path), t.String()))
		return
	}

	switch t.Kind() {
	case reflect.Pointer:
		describeType(t.Elem(), path, visiting, out)
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			*out = append(*out, fmt.Sprintf("%s bytes", pathOrRoot(path)))
			return
		}

		describeType(t.Elem(), path+"[]", visiting, out)
	case reflect.Map:
		*out = append(*out, fmt.Sprintf("%s map[%s]", pathOrRoot(path), t.Key().Kind()))
		describeType(t.Elem(), path+"{}", visiting, out)
	case reflect.Struct:
		if visiting[t] {
			*out = append(*out, fmt.Sprintf("%s recursive %s", pathOrRoot(path), t.String()))
			return
		}

		visiting[t] = true
		defer delete(visiting, t)

		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() && !f.Anonymous {
				continue
			}

			name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" && opts == "" {
				continue
			}

			if f.Anonymous && name == "" {
				describeType(f.Type, path, visiting, out)
				continue
			}

			if name == "" {
				name = f.Name
			}

			describeType(f.Type, joinPath(path, name), visiting, out)
		}
	default:
		*out = append(*out, fmt.Sprintf("%s %s", pathOrRoot(path), t.Kind()))
	}
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}

	return path + "." + name
}

func pathOrRoot(path string) string {
	if path == "" {
		return "."
	}

	return path
}

func currentPayloadShapes(t *testing.T) map[string]payloadShape {
	t.Helper()

	out := map[string]payloadShape{}

	for _, name := range commandOperations() {
		h, _ := commandHandlerFor(name)
		if h.payload == nil {
			t.Fatalf("operation %q does not record its payload type", name)
		}

		out[name] = payloadShape{schema: h.minSchema, fields: describePayload(h.payload)}
	}

	return out
}

func readPayloadShapes(t *testing.T) map[string]payloadShape {
	t.Helper()

	f, err := os.Open(goldenPayloadsPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		t.Fatalf("open %s: %v", goldenPayloadsPath, err)
	}

	defer func() { _ = f.Close() }()

	out := map[string]payloadShape{}

	var current string

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()

		if field, ok := strings.CutPrefix(line, "  "); ok {
			shape := out[current]
			shape.fields = append(shape.fields, field)
			out[current] = shape

			continue
		}

		var schema string

		if _, err := fmt.Sscanf(line, "op %s schema %s", &current, &schema); err != nil {
			t.Fatalf("%s: malformed line %q", goldenPayloadsPath, line)
		}

		n, err := strconv.Atoi(schema)
		if err != nil {
			t.Fatalf("%s: malformed schema in %q", goldenPayloadsPath, line)
		}

		out[current] = payloadShape{schema: n}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", goldenPayloadsPath, err)
	}

	return out
}

func formatPayloadShapes(shapes map[string]payloadShape) string {
	var b strings.Builder

	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range shapes {
			if !yield(k) {
				return
			}
		}
	}) {
		fmt.Fprintf(&b, "op %s schema %d\n", name, shapes[name].schema)

		for _, f := range shapes[name].fields {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}

	return b.String()
}

func TestCommandPayloadsChangeOnlyWithASchema(t *testing.T) {
	golden := readPayloadShapes(t)
	current := currentPayloadShapes(t)

	changed := false

	for name, shape := range current {
		old, ok := golden[name]
		if !ok {
			changed = true
			continue
		}

		if slices.Equal(old.fields, shape.fields) && old.schema == shape.schema {
			continue
		}

		changed = true

		if !slices.Equal(old.fields, shape.fields) && shape.schema <= old.schema {
			t.Errorf("the payload of %s changed without a new schema version: nodes that apply the old payload shape would diverge from nodes that apply the new one; give the operation a new name or raise its RequireSchema above %d\n  golden:  %s\n  current: %s",
				name, old.schema, strings.Join(old.fields, ", "), strings.Join(shape.fields, ", "))
		}
	}

	for name := range golden {
		if _, ok := current[name]; !ok {
			changed = true
		}
	}

	if t.Failed() || !changed {
		return
	}

	if !*updateSchemaGolden {
		t.Fatalf("command payloads differ from %s; regenerate with: go test ./internal/db/ -run TestCommandPayloadsChangeOnlyWithASchema -update", goldenPayloadsPath)
	}

	if err := os.WriteFile(goldenPayloadsPath, []byte(formatPayloadShapes(current)), 0o644); err != nil {
		t.Fatalf("write %s: %v", goldenPayloadsPath, err)
	}
}
