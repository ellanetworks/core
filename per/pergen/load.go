// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pergen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

type loaded struct {
	fset  *token.FileSet
	files []*ast.File
	types *types.Package
}

func load(ctx context.Context, dir, output string) (*loaded, error) {
	bp, err := build.ImportDir(dir, 0)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", dir, err)
	}

	fset := token.NewFileSet()

	var files []*ast.File

	for _, name := range bp.GoFiles {
		if name == output {
			continue
		}

		f, err := parser.ParseFile(fset, filepath.Join(bp.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}

		files = append(files, f)
	}

	exports, err := listExports(ctx, bp.Dir)
	if err != nil {
		return nil, err
	}

	var typeErrs []error

	conf := types.Config{
		Importer: importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
			file, ok := exports[path]
			if !ok || file == "" {
				return nil, fmt.Errorf("no export data for %s", path)
			}

			return os.Open(filepath.Clean(file))
		}),
		Error: func(err error) {
			if !pendingMethodError(err) {
				typeErrs = append(typeErrs, err)
			}
		},
	}

	pkg, _ := conf.Check(bp.ImportPath, fset, files, nil)
	if len(typeErrs) > 0 {
		return nil, fmt.Errorf("type-check %s: %w", bp.ImportPath, errors.Join(typeErrs...))
	}

	return &loaded{fset: fset, files: files, types: pkg}, nil
}

func listExports(ctx context.Context, dir string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-export", "-deps", "-json=ImportPath,Export", ".")
	cmd.Dir = dir

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, runErr := cmd.Output()
	exports := make(map[string]string)

	for dec := json.NewDecoder(bytes.NewReader(out)); ; {
		var p struct {
			ImportPath string
			Export     string
		}

		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go list: %w", err)
		}

		exports[p.ImportPath] = p.Export
	}

	if len(exports) == 0 && runErr != nil {
		return nil, fmt.Errorf("go list: %w: %s", runErr, stderr.Bytes())
	}

	return exports, nil
}

var generatedMethod = `(?:MarshalPER|UnmarshalPER|Diagnose|Name|Equal)`

var pendingMethodPatterns = []*regexp.Regexp{
	regexp.MustCompile(`has no field or method ` + generatedMethod + `\b`),
	regexp.MustCompile(`\(missing method ` + generatedMethod + `\)`),
}

func pendingMethodError(err error) bool {
	for _, re := range pendingMethodPatterns {
		if re.MatchString(err.Error()) {
			return true
		}
	}

	return false
}
