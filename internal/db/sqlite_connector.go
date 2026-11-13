// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql/driver"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/mattn/go-sqlite3"
)

type sqliteConnector struct {
	dsn        string
	driver     *sqlite3.SQLiteDriver
	generation atomic.Uint64
	opening    sync.RWMutex
}

func newSQLiteConnector(dsn string) *sqliteConnector {
	return &sqliteConnector{dsn: dsn, driver: &sqlite3.SQLiteDriver{}}
}

func (c *sqliteConnector) Connect(context.Context) (driver.Conn, error) {
	c.opening.RLock()
	defer c.opening.RUnlock()

	generation := c.generation.Load()

	conn, err := c.driver.Open(c.dsn)
	if err != nil {
		return nil, err
	}

	sqliteConn, ok := conn.(*sqlite3.SQLiteConn)
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected sqlite driver conn type %T", conn)
	}

	return &generationConn{SQLiteConn: sqliteConn, generation: generation, connector: c}, nil
}

func (c *sqliteConnector) Driver() driver.Driver {
	return c.driver
}

func (c *sqliteConnector) retireConnections() {
	c.generation.Add(1)
}

func (c *sqliteConnector) blockConnects() func() {
	c.opening.Lock()

	return c.opening.Unlock
}

type generationConn struct {
	*sqlite3.SQLiteConn
	generation uint64
	connector  *sqliteConnector
}

func (c *generationConn) IsValid() bool {
	return c.generation == c.connector.generation.Load()
}

func (c *generationConn) ResetSession(context.Context) error {
	if !c.IsValid() {
		return driver.ErrBadConn
	}

	return nil
}

func rawSQLiteConn(raw any) (*sqlite3.SQLiteConn, error) {
	switch conn := raw.(type) {
	case *generationConn:
		return conn.SQLiteConn, nil
	case *sqlite3.SQLiteConn:
		return conn, nil
	default:
		return nil, fmt.Errorf("unexpected sqlite driver conn type %T", raw)
	}
}
