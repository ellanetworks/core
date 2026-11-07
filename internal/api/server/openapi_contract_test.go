// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type goField struct {
	structName string
	jsonName   string
	pointer    bool
	omitEmpty  bool
}

func packageStructFields(t *testing.T) []goField {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("could not list the package directory: %v", err)
	}

	fset := token.NewFileSet()

	var out []goField

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

			for _, field := range structType.Fields.List {
				if field.Tag == nil {
					continue
				}

				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("json")

				parts := strings.Split(tag, ",")
				if parts[0] == "" || parts[0] == "-" {
					continue
				}

				_, pointer := field.Type.(*ast.StarExpr)

				out = append(out, goField{
					structName: spec.Name.Name,
					jsonName:   parts[0],
					pointer:    pointer,
					omitEmpty:  slices.Contains(parts[1:], "omitempty"),
				})
			}

			return true
		})
	}

	return out
}

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

	for _, field := range packageStructFields(t) {
		schema, ok := spec.Components.Schemas[field.structName]
		if !ok || !slices.Contains(schema.Required, field.jsonName) {
			continue
		}

		if field.omitEmpty {
			t.Errorf(
				"%s.%s is required by the OpenAPI spec but tagged omitempty",
				field.structName, field.jsonName,
			)
		}
	}
}

func admitsNull(schema map[string]any) bool {
	switch declared := schema["type"].(type) {
	case string:
		if declared == "null" {
			return true
		}
	case []any:
		for _, entry := range declared {
			if name, ok := entry.(string); ok && name == "null" {
				return true
			}
		}
	}

	for _, keyword := range []string{"oneOf", "anyOf"} {
		branches, ok := schema[keyword].([]any)
		if !ok {
			continue
		}

		for _, branch := range branches {
			if nested, ok := branch.(map[string]any); ok && admitsNull(nested) {
				return true
			}
		}
	}

	return false
}

func TestSpecDeclaresNullForEveryNullableField(t *testing.T) {
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}

	if err := yaml.Unmarshal(openapiSpec, &spec); err != nil {
		t.Fatalf("could not parse the embedded OpenAPI spec: %v", err)
	}

	for _, field := range packageStructFields(t) {
		schema, ok := spec.Components.Schemas[field.structName]
		if !ok {
			continue
		}

		property, ok := schema.Properties[field.jsonName]
		if !ok {
			continue
		}

		marshalsNull := field.pointer && !field.omitEmpty
		if marshalsNull == admitsNull(property) {
			continue
		}

		if marshalsNull {
			t.Errorf(
				"%s.%s marshals as null when the pointer is nil, but its schema does not admit null",
				field.structName, field.jsonName,
			)
		} else {
			t.Errorf(
				"%s.%s can never marshal as null, but its schema admits null",
				field.structName, field.jsonName,
			)
		}
	}
}

func TestSpecDeclaresEveryStructField(t *testing.T) {
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}

	if err := yaml.Unmarshal(openapiSpec, &spec); err != nil {
		t.Fatalf("could not parse the embedded OpenAPI spec: %v", err)
	}

	for _, field := range packageStructFields(t) {
		schema, ok := spec.Components.Schemas[field.structName]
		if !ok || len(schema.Properties) == 0 {
			continue
		}

		if _, ok := schema.Properties[field.jsonName]; !ok {
			t.Errorf("%s.%s is not declared in the OpenAPI schema %s", field.structName, field.jsonName, field.structName)
		}
	}
}

func TestSpecDocumentsEveryRoute(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}

	if err := yaml.Unmarshal(openapiSpec, &spec); err != nil {
		t.Fatalf("could not parse the embedded OpenAPI spec: %v", err)
	}

	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("could not read server.go: %v", err)
	}

	routes := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/api/v1/[^"]*)"`).FindAllStringSubmatch(string(source), -1)
	if len(routes) == 0 {
		t.Fatal("found no routes in server.go")
	}

	for _, route := range routes {
		method, path := strings.ToLower(route[1]), route[2]
		if path == "/api/v1/openapi.yaml" {
			continue
		}

		if _, ok := spec.Paths[path][method]; !ok {
			t.Errorf("%s %s is not documented in the OpenAPI spec", route[1], path)
		}
	}
}
