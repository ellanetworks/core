// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func TestReaderPoolServesReadsWhileWriterIsHeld(t *testing.T) {
	ctx := context.Background()

	database, err := NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), DBFilename))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	holding := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		_, err := database.applyLocalTx(ctx, "hold", func(ctx context.Context) (any, error) {
			if _, err := database.runner(ctx).ExecContext(ctx, "UPDATE "+NetworkSlicesTableName+" SET sst = 2 WHERE name = ?", InitialSliceName); err != nil {
				return nil, err
			}

			close(holding)
			<-release

			return nil, nil
		})
		done <- err
	}()

	<-holding

	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	slice, err := database.GetNetworkSlice(readCtx, InitialSliceName)
	if err != nil {
		t.Fatalf("read while the writer is held: %v", err)
	}

	if slice.Sst != InitialSliceSst {
		t.Fatalf("read saw the held transaction's uncommitted write: sst = %d", slice.Sst)
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatalf("held transaction: %v", err)
	}

	slice, err = database.GetNetworkSlice(ctx, InitialSliceName)
	if err != nil {
		t.Fatalf("read after commit: %v", err)
	}

	if slice.Sst != 2 {
		t.Fatalf("read after commit: sst = %d, want 2", slice.Sst)
	}
}

func TestReaderPoolRejectsWrites(t *testing.T) {
	ctx := context.Background()

	database, err := NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), DBFilename))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	_, err = database.reader().ExecContext(ctx, "DELETE FROM "+NetworkSlicesTableName)

	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code != sqlite3.ErrReadonly {
		t.Fatalf("write through the reader pool: got %v, want SQLITE_READONLY", err)
	}
}

func TestReopenRetiresReaderPoolConnections(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), DBFilename)

	database, err := NewDatabaseWithoutRaft(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	writer := database.PlainDB()

	if _, err := writer.ExecContext(ctx, "CREATE TABLE probe (v TEXT NOT NULL)"); err != nil {
		t.Fatalf("create probe: %v", err)
	}

	if _, err := writer.ExecContext(ctx, "INSERT INTO probe(v) VALUES ('before')"); err != nil {
		t.Fatalf("insert probe: %v", err)
	}

	staged := dbPath + ".staged"

	if _, err := writer.ExecContext(ctx, "VACUUM INTO ?", staged); err != nil {
		t.Fatalf("stage snapshot: %v", err)
	}

	stagedDB, err := openSQLiteConnection(ctx, staged)
	if err != nil {
		t.Fatalf("open staged: %v", err)
	}

	if _, err := stagedDB.ExecContext(ctx, "UPDATE probe SET v = 'after'"); err != nil {
		t.Fatalf("update staged: %v", err)
	}

	if err := stagedDB.Close(); err != nil {
		t.Fatalf("close staged: %v", err)
	}

	reader := database.reader()

	warm := make([]*sql.Conn, 0, readConns)

	for range readConns {
		c, err := reader.Conn(ctx)
		if err != nil {
			t.Fatalf("warm reader pool: %v", err)
		}

		if got := readProbe(t, ctx, c); got != "before" {
			t.Fatalf("reader before swap = %q, want %q", got, "before")
		}

		warm = append(warm, c)
	}

	inFlight := warm[0]

	for _, c := range warm[1:] {
		if err := c.Close(); err != nil {
			t.Fatalf("release reader conn: %v", err)
		}
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}

	if err := os.Rename(staged, dbPath); err != nil {
		t.Fatalf("swap in snapshot: %v", err)
	}

	if err := database.Reopen(ctx); err != nil {
		t.Fatalf("Reopen: %v", err)
	}

	if got := readProbe(t, ctx, inFlight); got != "before" {
		t.Fatalf("in-flight reader conn after Reopen = %q, want %q", got, "before")
	}

	if err := inFlight.Close(); err != nil {
		t.Fatalf("release in-flight reader conn: %v", err)
	}

	for range 2 * readConns {
		if got := readProbe(t, ctx, reader); got != "after" {
			t.Fatalf("reader pool after Reopen = %q, want %q", got, "after")
		}
	}
}

func TestApplyFunctionsNeverReadThroughTheReaderPool(t *testing.T) {
	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	calls := map[string]map[string]bool{}
	roots := map[string]bool{}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(sel.Sel.Name, "apply") {
				return true
			}

			if paren, ok := sel.X.(*ast.ParenExpr); ok {
				if star, ok := paren.X.(*ast.StarExpr); ok {
					if id, ok := star.X.(*ast.Ident); ok && id.Name == "Database" {
						roots[sel.Sel.Name] = true
					}
				}
			}

			return true
		})

		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}

			callees := map[string]bool{}

			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				switch fn := call.Fun.(type) {
				case *ast.SelectorExpr:
					if x, ok := fn.X.(*ast.Ident); ok && x.Name == "db" {
						callees[fn.Sel.Name] = true
					}
				case *ast.Ident:
					callees[fn.Name] = true
				}

				return true
			})

			calls[fd.Name.Name] = callees
		}
	}

	if len(roots) < 100 {
		t.Fatalf("found only %d apply functions; the root matcher no longer finds the registrations", len(roots))
	}

	for root := range roots {
		parent := map[string]string{root: ""}
		queue := []string{root}

		for len(queue) > 0 {
			fn := queue[0]
			queue = queue[1:]

			for callee := range calls[fn] {
				if callee == "readQuerier" {
					continue
				}

				if callee == "reader" {
					path := fn
					for p := parent[fn]; p != ""; p = parent[p] {
						path = p + " -> " + path
					}

					t.Errorf("%s reaches db.reader() via %s; apply functions must read through db.runner(ctx) or db.readQuerier(ctx)", root, path)

					continue
				}

				if _, seen := parent[callee]; !seen {
					parent[callee] = fn
					queue = append(queue, callee)
				}
			}
		}
	}
}
