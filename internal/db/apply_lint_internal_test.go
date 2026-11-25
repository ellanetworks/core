// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const modulePath = "github.com/ellanetworks/core"

var applyCallAllowlist = map[string]string{
	"db.applyTime":            "returns time.Now only on the capture path, where no applyInfo is in the context",
	"db.payloadID":            "generates a UUID only on the capture path, where no applyInfo is in the context",
	"db.applyIndexFloor":      "reads the raft applied index only on the capture path, where no applyInfo is in the context",
	"db.Database.runner":      "returns the apply transaction's connection whenever one is in the context, and a pool only on the capture path",
	"db.Database.readQuerier": "returns the apply transaction's connection whenever one is in the context, and a pool only on the capture path",
}

var uncodedApplySentinels = map[string]string{
	"db.errWriteQueueFull": "the proposer's own queue error; never returned by an apply function on the command path",
}

var rowidQueryAllowlist = map[string]string{
	"getNewestLeaseInPoolSQL": "next-fit cursor used only on the capture path, below the command schema",
}

type lintFunc struct {
	pkg  *lintPackage
	file *ast.File
	decl *ast.FuncDecl
	name string
}

type lintPackage struct {
	importPath string
	name       string
	files      []*ast.File
	info       *types.Info
	types      *types.Package
}

type lintProgram struct {
	fset     *token.FileSet
	packages map[string]*lintPackage
	funcs    map[string]*lintFunc
}

var (
	lintProgramOnce sync.Once
	lintProgramVal  *lintProgram
	lintProgramErr  error
)

type listedPackage struct {
	ImportPath string
	Dir        string
	Export     string
	GoFiles    []string
}

func loadLintProgram(t *testing.T) *lintProgram {
	t.Helper()

	lintProgramOnce.Do(func() {
		lintProgramVal, lintProgramErr = buildLintProgram()
	})

	if lintProgramErr != nil {
		t.Fatalf("load packages: %v", lintProgramErr)
	}

	return lintProgramVal
}

func buildLintProgram() (*lintProgram, error) {
	cmd := exec.CommandContext(context.Background(), "go", "list", "-export", "-deps", "-json=ImportPath,Dir,Export,GoFiles", ".")

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("go list: %v: %s", err, exitErr.Stderr)
		}

		return nil, fmt.Errorf("go list: %w", err)
	}

	exports := map[string]string{}

	var module []listedPackage

	dec := json.NewDecoder(bytes.NewReader(out))

	for dec.More() {
		var lp listedPackage
		if err := dec.Decode(&lp); err != nil {
			return nil, err
		}

		exports[lp.ImportPath] = lp.Export

		if lp.ImportPath == modulePath || strings.HasPrefix(lp.ImportPath, modulePath+"/") {
			module = append(module, lp)
		}
	}

	p := &lintProgram{fset: token.NewFileSet(), packages: map[string]*lintPackage{}, funcs: map[string]*lintFunc{}}

	imp := importer.ForCompiler(p.fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok || file == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}

		return os.Open(file)
	})

	for _, lp := range module {
		pkg := &lintPackage{importPath: lp.ImportPath}

		for _, name := range lp.GoFiles {
			f, err := parser.ParseFile(p.fset, filepath.Join(lp.Dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, err
			}

			pkg.files = append(pkg.files, f)
			pkg.name = f.Name.Name
		}

		pkg.info = &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
		conf := types.Config{Importer: imp, Error: func(error) {}}
		pkg.types, _ = conf.Check(lp.ImportPath, p.fset, pkg.files, pkg.info)
		p.packages[lp.ImportPath] = pkg

		for _, f := range pkg.files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}

				obj, ok := pkg.info.Defs[fd.Name].(*types.Func)
				if !ok {
					continue
				}

				p.funcs[obj.FullName()] = &lintFunc{pkg: pkg, file: f, decl: fd, name: shortFuncName(obj)}
			}
		}
	}

	if p.packages[modulePath+"/internal/db"] == nil {
		return nil, errors.New("package internal/db not loaded")
	}

	return p, nil
}

func shortFuncName(fn *types.Func) string {
	pkg := ""
	if fn.Pkg() != nil {
		pkg = fn.Pkg().Name() + "."
	}

	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		recv := sig.Recv().Type()
		if ptr, ok := recv.(*types.Pointer); ok {
			recv = ptr.Elem()
		}

		if named, ok := recv.(*types.Named); ok {
			return pkg + named.Obj().Name() + "." + fn.Name()
		}
	}

	return pkg + fn.Name()
}

func funcKey(obj types.Object) (string, bool) {
	fn, ok := obj.(*types.Func)
	if !ok {
		return "", false
	}

	return fn.Origin().FullName(), true
}

var leaderSideOptions = map[string]bool{"checkOnLeader": true, "skipWhenUnchanged": true}

func (p *lintProgram) applyRoots() []*lintFunc {
	dbPkg := p.packages[modulePath+"/internal/db"]

	registrars := map[string]bool{
		"registerChangesetOp": true, "registerChangesetOpReturning": true,
		"registerIntentOp": true, "registerIntentOpReturning": true,
		"asCommand": true, "withoutResult": true,
	}

	seen := map[*lintFunc]bool{}

	var roots []*lintFunc

	for _, f := range dbPkg.files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !registrars[calleeName(call.Fun)] {
				return true
			}

			for _, arg := range call.Args {
				ast.Inspect(arg, func(n ast.Node) bool {
					if opt, ok := n.(*ast.CallExpr); ok && leaderSideOptions[calleeName(opt.Fun)] {
						return false
					}

					id, ok := n.(*ast.Ident)
					if !ok {
						return true
					}

					key, ok := funcKey(dbPkg.info.Uses[id])
					if !ok {
						return true
					}

					if fn := p.funcs[key]; fn != nil && !seen[fn] && !registrars[fn.decl.Name.Name] {
						seen[fn] = true
						roots = append(roots, fn)
					}

					return true
				})
			}

			return true
		})
	}

	return roots
}

func calleeName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return calleeName(e.X)
	case *ast.IndexListExpr:
		return calleeName(e.X)
	default:
		return ""
	}
}

type reachedFunc struct {
	fn   *lintFunc
	path []string
}

func (p *lintProgram) reachable(roots []*lintFunc) []reachedFunc {
	visited := map[*lintFunc]bool{}

	var (
		out   []reachedFunc
		queue []reachedFunc
	)

	for _, r := range roots {
		visited[r] = true
		queue = append(queue, reachedFunc{fn: r, path: []string{r.name}})
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		out = append(out, cur)

		if _, allowed := applyCallAllowlist[cur.fn.name]; allowed {
			continue
		}

		ast.Inspect(cur.fn.decl.Body, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}

			key, ok := funcKey(cur.fn.pkg.info.Uses[id])
			if !ok {
				return true
			}

			if t := p.funcs[key]; t != nil && !visited[t] {
				visited[t] = true
				queue = append(queue, reachedFunc{fn: t, path: append(slices.Clone(cur.path), t.name)})
			}

			return true
		})
	}

	return out
}

func forbiddenApplyUse(obj types.Object) string {
	if obj == nil || obj.Pkg() == nil {
		return ""
	}

	pkg, name := obj.Pkg().Path(), obj.Name()

	if v, ok := obj.(*types.Var); ok {
		switch {
		case v.IsField() && name == "raftManager":
			return "Database.raftManager"
		case !v.IsField() && v.Parent() == v.Pkg().Scope() && (pkg == "math/rand" || pkg == "math/rand/v2" || pkg == "crypto/rand"):
			return pkg + "." + name
		}

		return ""
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		return ""
	}

	if pkg == modulePath+"/internal/db" {
		switch shortFuncName(fn) {
		case "db.Database.reader", "db.Database.conn", "db.Database.PlainDB", "db.Database.writer":
			return shortFuncName(fn) + ", which reads outside the apply transaction"
		}
	}

	switch {
	case pkg == "time" && (name == "Now" || name == "Since" || name == "Until"):
		return "time." + name
	case pkg == "math/rand" || pkg == "math/rand/v2" || pkg == "crypto/rand":
		return pkg + "." + name
	case pkg == "github.com/google/uuid" && strings.HasPrefix(name, "New"):
		return "uuid." + name
	case pkg == "os" && (name == "Getenv" || name == "LookupEnv" || name == "Environ"):
		return "os." + name
	case pkg == "golang.org/x/crypto/bcrypt":
		return "bcrypt." + name
	case name == "AppliedIndex":
		return shortFuncName(obj.(*types.Func))
	}

	return ""
}

func TestApplyFunctionsAreDeterministic(t *testing.T) {
	p := loadLintProgram(t)

	roots := p.applyRoots()
	if len(roots) < 100 {
		t.Fatalf("found %d registered apply functions; the registration scan is broken", len(roots))
	}

	var violations []string

	for _, r := range p.reachable(roots) {
		if _, allowed := applyCallAllowlist[r.fn.name]; allowed {
			continue
		}

		ast.Inspect(r.fn.decl.Body, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}

			if what := forbiddenApplyUse(r.fn.pkg.info.Uses[id]); what != "" {
				violations = append(violations, fmt.Sprintf("%s: %s uses %s, reached from a registered apply function: %s",
					p.fset.Position(id.Pos()), r.fn.name, what, strings.Join(r.path, " -> ")))
			}

			return true
		})
	}

	sort.Strings(violations)

	for _, v := range violations {
		t.Error(v)
	}

	if len(violations) > 0 {
		t.Log("apply functions run on every node and must give the same result everywhere: take time from applyTime(ctx), IDs and randomness from the payload, and the log index from applyInfoFrom(ctx)")
	}
}

var lintErrorType = types.Universe.Lookup("error").Type()

func sentinelName(info *types.Info, e ast.Expr) string {
	var id *ast.Ident

	switch e := e.(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		return ""
	}

	v, ok := info.Uses[id].(*types.Var)
	if !ok || v.IsField() || v.Pkg() == nil || v.Parent() != v.Pkg().Scope() || !types.Identical(v.Type(), lintErrorType) {
		return ""
	}

	return v.Pkg().Name() + "." + v.Name()
}

func resultCodeSentinels(t *testing.T, p *lintProgram) map[string]bool {
	t.Helper()

	dbPkg := p.packages[modulePath+"/internal/db"]
	out := map[string]bool{"db.ErrInvalidCommand": true}

	for _, f := range dbPkg.files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || len(lit.Elts) != 2 {
				return true
			}

			code, ok := lit.Elts[0].(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(code.Sel.Name, "ForwardCode") {
				return true
			}

			if name := sentinelName(dbPkg.info, lit.Elts[1]); name != "" {
				out[name] = true
			}

			return true
		})
	}

	if len(out) < 20 {
		t.Fatalf("found %d entries in the result-code table; the table scan is broken", len(out))
	}

	return out
}

func TestApplyFunctionSentinelsHaveResultCodes(t *testing.T) {
	p := loadLintProgram(t)
	coded := resultCodeSentinels(t, p)

	var violations []string

	for _, r := range p.reachable(p.applyRoots()) {
		if r.fn.pkg.importPath != modulePath+"/internal/db" {
			continue
		}

		ast.Inspect(r.fn.decl.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}

			for _, result := range ret.Results {
				ast.Inspect(result, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						if name := calleeName(call.Fun); name == "invalidCommand" {
							return false
						}

						if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Is" || sel.Sel.Name == "As") {
							return false
						}
					}

					expr, ok := n.(ast.Expr)
					if !ok {
						return true
					}

					name := sentinelName(r.fn.pkg.info, expr)
					if name == "" || coded[name] {
						return true
					}

					if _, uncoded := uncodedApplySentinels[name]; uncoded {
						return true
					}

					violations = append(violations, fmt.Sprintf("%s: %s returns %s, which has no result code, reached from a registered apply function: %s",
						p.fset.Position(expr.Pos()), r.fn.name, name, strings.Join(r.path, " -> ")))

					return false
				})
			}

			return true
		})
	}

	sort.Strings(violations)

	for _, v := range violations {
		t.Error(v)
	}

	if len(violations) > 0 {
		t.Log("a follower that forwards the write gets a generic error instead of this sentinel: add the sentinel to the forward-code table in operations.go, or to uncodedApplySentinels")
	}
}

type sqlString struct {
	pos   token.Pos
	owner string
	text  string
}

func packageSQLStrings(p *lintProgram) []sqlString {
	dbPkg := p.packages[modulePath+"/internal/db"]
	consts := map[string]string{}

	for range 8 {
		for _, f := range dbPkg.files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok {
					continue
				}

				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					for i, n := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}

						if s, ok := foldString(vs.Values[i], consts, false); ok {
							consts[n.Name] = s
						}
					}
				}
			}
		}
	}

	var out []sqlString

	for _, f := range dbPkg.files {
		var visit func(n ast.Node, owner string)

		visit = func(n ast.Node, owner string) {
			ast.Inspect(n, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncDecl:
					if n.Body != nil {
						visit(n.Body, n.Name.Name)
					}

					return false
				case *ast.ValueSpec:
					for i, name := range n.Names {
						if i < len(n.Values) {
							visit(n.Values[i], name.Name)
						}
					}

					return false
				case *ast.BinaryExpr, *ast.BasicLit:
					expr, _ := n.(ast.Expr)
					if s, ok := foldString(expr, consts, true); ok {
						out = append(out, sqlString{pos: expr.Pos(), owner: owner, text: s})
						return false
					}
				}

				return true
			})
		}

		visit(f, "")
	}

	return out
}

func foldString(e ast.Expr, consts map[string]string, lenient bool) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}

		s, err := strconv.Unquote(e.Value)

		return s, err == nil
	case *ast.Ident:
		if s, ok := consts[e.Name]; ok {
			return s, true
		}

		if lenient {
			return " ?" + e.Name + "? ", true
		}

		return "", false
	case *ast.ParenExpr:
		return foldString(e.X, consts, lenient)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}

		a, ok := foldString(e.X, consts, lenient)
		if !ok {
			return "", false
		}

		b, ok := foldString(e.Y, consts, lenient)
		if !ok {
			return "", false
		}

		if lenient && strings.HasPrefix(a, " ?") && strings.HasPrefix(b, " ?") && strings.HasSuffix(a, "? ") && strings.HasSuffix(b, "? ") {
			return "", false
		}

		return a + b, true
	default:
		if lenient {
			return " ?expr? ", true
		}

		return "", false
	}
}

func uniqueKeys(t *testing.T) map[string][][]string {
	t.Helper()

	database := newDatabaseAtVersion(t, SchemaVersion())
	ctx := context.Background()

	rows, err := database.writer().QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}

	var tables []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}

		tables = append(tables, name)
	}

	_ = rows.Close()

	keys := map[string][][]string{}

	for _, table := range tables {
		var pk []string

		nullable := map[string]bool{}

		cols, err := database.writer().QueryContext(ctx, "SELECT name, \"notnull\", pk FROM pragma_table_info(?) ORDER BY pk, cid", table)
		if err != nil {
			t.Fatalf("table_info %s: %v", table, err)
		}

		for cols.Next() {
			var (
				c          string
				notNull, k int
			)

			if err := cols.Scan(&c, &notNull, &k); err != nil {
				t.Fatalf("scan: %v", err)
			}

			if k > 0 {
				pk = append(pk, strings.ToLower(c))
			} else if notNull == 0 {
				nullable[strings.ToLower(c)] = true
			}
		}

		_ = cols.Close()

		if len(pk) > 0 {
			keys[table] = append(keys[table], pk)
		}

		idx, err := database.writer().QueryContext(ctx, "SELECT name FROM pragma_index_list(?) WHERE \"unique\" = 1 AND partial = 0", table)
		if err != nil {
			t.Fatalf("index_list %s: %v", table, err)
		}

		var indexes []string

		for idx.Next() {
			var name string
			if err := idx.Scan(&name); err != nil {
				t.Fatalf("scan: %v", err)
			}

			indexes = append(indexes, name)
		}

		_ = idx.Close()

		for _, name := range indexes {
			var key []string

			ic, err := database.writer().QueryContext(ctx, "SELECT name FROM pragma_index_info(?) ORDER BY seqno", name)
			if err != nil {
				t.Fatalf("index_info %s: %v", name, err)
			}

			for ic.Next() {
				var c *string
				if err := ic.Scan(&c); err != nil {
					t.Fatalf("scan: %v", err)
				}

				if c == nil || nullable[strings.ToLower(*c)] {
					key = nil

					break
				}

				key = append(key, strings.ToLower(*c))
			}

			_ = ic.Close()

			if len(key) > 0 {
				keys[table] = append(keys[table], key)
			}
		}
	}

	return keys
}

var (
	limitKeyword   = regexp.MustCompile(`(?i)\bLIMIT\s+(\?\d*|\d+)`)
	orderedLimit   = regexp.MustCompile(`(?is)ORDER\s+BY\s+(.*?)\s+LIMIT\s+(\?\d*|\d+)`)
	orderTermNoise = regexp.MustCompile(`(?i)\s+(ASC|DESC|COLLATE\s+\w+)\b`)
	rowidKeyword   = regexp.MustCompile(`(?i)\b(_rowid_|rowid|oid)\b`)
	withoutRowid   = regexp.MustCompile(`(?i)WITHOUT\s+ROWID`)
	groupByClause  = regexp.MustCompile(`(?is)GROUP\s+BY\s+(.*?)\s+(?:ORDER|HAVING|LIMIT)\b`)
	equalityTerm   = regexp.MustCompile(`(?i)(?:WHERE|AND)\s+(?:\w+\.)?(\w+)\s*==?\s*\?\d*`)
)

func orderTerms(clause string) []string {
	var terms []string

	for _, term := range strings.Split(clause, ",") {
		term = strings.TrimSpace(orderTermNoise.ReplaceAllString(term, ""))
		if i := strings.LastIndex(term, "."); i >= 0 {
			term = term[i+1:]
		}

		terms = append(terms, strings.ToLower(strings.Trim(term, `"`+"`")))
	}

	return terms
}

var queriedTable = regexp.MustCompile(`(?i)\b(?:FROM|JOIN|UPDATE|INTO)\s+["` + "`" + `]?(\w+|%s|\?\w+\?)`)

func tableKeys(text string, keys map[string][][]string) [][]string {
	var (
		out     [][]string
		unknown bool
	)

	for _, m := range queriedTable.FindAllStringSubmatch(text, -1) {
		table := strings.ToLower(m[1])
		if tk, ok := keys[table]; ok {
			out = append(out, tk...)

			continue
		}

		if table == "%s" || strings.HasPrefix(table, "?") {
			unknown = true
		}
	}

	if unknown {
		for _, tk := range keys {
			out = append(out, tk...)
		}
	}

	return out
}

func queryKeys(text string, keys map[string][][]string) ([][]string, []string) {
	var (
		local [][]string
		fixed []string
	)

	for _, m := range groupByClause.FindAllStringSubmatch(text, -1) {
		local = append(local, orderTerms(m[1]))
	}

	for _, m := range equalityTerm.FindAllStringSubmatch(text, -1) {
		fixed = append(fixed, strings.ToLower(m[1]))
	}

	return append(tableKeys(text, keys), local...), fixed
}

func totalOrder(terms []string, keys [][]string, fixed []string) bool {
	last := terms[len(terms)-1]

	for _, key := range keys {
		if !slices.Contains(key, last) {
			continue
		}

		covered := true

		for _, c := range key {
			if !slices.Contains(terms, c) && !slices.Contains(fixed, c) {
				covered = false
				break
			}
		}

		if covered {
			return true
		}
	}

	return false
}

func TestBoundedQueriesHaveATotalOrder(t *testing.T) {
	p := loadLintProgram(t)
	keys := uniqueKeys(t)

	for _, s := range packageSQLStrings(p) {
		limits := limitKeyword.FindAllStringIndex(s.text, -1)
		if len(limits) == 0 {
			continue
		}

		if _, allowed := rowidQueryAllowlist[s.owner]; allowed {
			continue
		}

		queryKeys, fixed := queryKeys(s.text, keys)

		ordered := orderedLimit.FindAllStringSubmatch(s.text, -1)
		if len(ordered) != len(limits) {
			t.Errorf("%s (%s): a LIMIT without an ORDER BY: %s", p.fset.Position(s.pos), s.owner, strings.Join(strings.Fields(s.text), " "))
			continue
		}

		for _, m := range ordered {
			if !totalOrder(orderTerms(m[1]), queryKeys, fixed) {
				t.Errorf("%s (%s): ORDER BY %s does not end with a unique key, so the rows a LIMIT keeps depend on physical row order", p.fset.Position(s.pos), s.owner, strings.Join(strings.Fields(m[1]), " "))
			}
		}
	}
}

func TestReplicatedQueriesDoNotUseRowid(t *testing.T) {
	p := loadLintProgram(t)

	replicated := map[string]bool{}
	for _, table := range replicatedTables {
		replicated[table] = true
	}

	word := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|\?[A-Za-z0-9_]+\?`)
	used := map[string]bool{}

	for _, s := range packageSQLStrings(p) {
		text := withoutRowid.ReplaceAllString(s.text, "")
		if !rowidKeyword.MatchString(text) {
			continue
		}

		var touched []string

		for _, w := range word.FindAllString(text, -1) {
			if replicated[w] || strings.HasPrefix(w, "?") {
				touched = append(touched, w)
			}
		}

		if len(touched) == 0 {
			continue
		}

		if _, allowed := rowidQueryAllowlist[s.owner]; allowed {
			used[s.owner] = true
			continue
		}

		t.Errorf("%s (%s): uses rowid on %s; rowids differ between nodes under command replication: %s",
			p.fset.Position(s.pos), s.owner, strings.Join(slices.Compact(touched), ", "), strings.Join(strings.Fields(s.text), " "))
	}

	for owner := range rowidQueryAllowlist {
		if !used[owner] {
			t.Errorf("rowidQueryAllowlist[%q] matches no query; remove it", owner)
		}
	}
}
