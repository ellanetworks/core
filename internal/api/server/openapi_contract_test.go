// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSpecRequiredFieldsAreNeverOmitEmpty(t *testing.T) {
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Required []string `yaml:"required"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}

	if err := yaml.Unmarshal(openapiSpec, &spec); err != nil {
		t.Fatalf("could not parse the embedded OpenAPI spec: %v", err)
	}

	required := make(map[string]map[string]bool, len(spec.Components.Schemas))
	for name, schema := range spec.Components.Schemas {
		fields := make(map[string]bool, len(schema.Required))
		for _, field := range schema.Required {
			fields[field] = true
		}

		required[name] = fields
	}

	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("could not list the package directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("could not parse %s: %v", name, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}

			structType, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			fields, ok := required[spec.Name.Name]
			if !ok {
				return true
			}

			for _, field := range structType.Fields.List {
				if field.Tag == nil {
					continue
				}

				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("json")

				parts := strings.Split(tag, ",")
				if len(parts) < 2 || !fields[parts[0]] {
					continue
				}

				for _, opt := range parts[1:] {
					if opt == "omitempty" {
						t.Errorf(
							"%s.%s is required by the OpenAPI spec but tagged omitempty",
							spec.Name.Name, parts[0],
						)
					}
				}
			}

			return true
		})
	}
}
