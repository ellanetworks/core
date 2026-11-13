// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const IPLeasesTableName = "ip_leases"

const (
	createLeaseSQL              = "INSERT INTO " + IPLeasesTableName + " (id, poolID, poolType, addressBin, imsi, sessionID, type, createdAt, nodeID) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	getDynamicLeaseBySessionSQL = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE +poolID==? AND poolType==? AND imsi==? AND +sessionID==? AND type='dynamic'"
	getLeaseBySessionSQL        = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE +poolID==? AND poolType==? AND +sessionID==? AND imsi==?"
	getActiveLeaseByAddressSQL  = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE poolID==? AND addressBin==? AND sessionID IS NOT NULL"
	updateLeaseSessionSQL       = "UPDATE " + IPLeasesTableName + " SET sessionID=? WHERE id==?"
	updateLeaseNodeSQL          = "UPDATE " + IPLeasesTableName + " SET nodeID=?, sessionID=? WHERE id==?"
	deleteLeaseSQL              = "DELETE FROM " + IPLeasesTableName + " WHERE id==? AND type='dynamic'"
	deleteAllDynamicLeasesSQL   = "DELETE FROM " + IPLeasesTableName + " WHERE type='dynamic'"
	deleteDynLeasesByNodeSQL    = "DELETE FROM " + IPLeasesTableName + " WHERE type='dynamic' AND nodeID==?"
	listActiveLeasesSQL         = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE sessionID IS NOT NULL"
	listActiveLeasesByNodeSQL   = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE sessionID IS NOT NULL AND nodeID==?"
	listLeasesByPoolSQL         = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE poolID==? AND poolType==?"
	listLeaseAddressRangeSQL    = "SELECT addressBin FROM " + IPLeasesTableName + " WHERE poolID==? AND poolType==? AND addressBin >= ? AND addressBin <= ? ORDER BY addressBin"
	getNewestLeaseInPoolSQL     = "SELECT addressBin FROM " + IPLeasesTableName + " WHERE poolID==? AND poolType==? ORDER BY rowid DESC LIMIT 1"
	countLeasesByPoolSQL        = "SELECT COUNT(*) FROM " + IPLeasesTableName + " WHERE poolID==? AND poolType==?"
	countIPv4LeasesByPoolSQL    = "SELECT COUNT(*) FROM " + IPLeasesTableName + " WHERE poolID==? AND poolType==? AND substr(addressBin, 1, 12) = X'00000000000000000000FFFF'"
	countIPv6LeasesByPoolSQL    = "SELECT COUNT(*) FROM " + IPLeasesTableName + " WHERE poolID==? AND poolType==? AND substr(addressBin, 1, 12) != X'00000000000000000000FFFF'"
	countActiveLeasesSQL        = "SELECT COUNT(*) FROM " + IPLeasesTableName + " WHERE sessionID IS NOT NULL"
	countLeasesByIMSISQL        = "SELECT COUNT(*) FROM " + IPLeasesTableName + " WHERE imsi==?"
	listLeasesByPoolPageSQL     = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE poolID==? AND poolType==? ORDER BY addressBin LIMIT ? OFFSET ?"
	listAllLeasesSQL            = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName
	getStaticLeaseSQL           = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE poolID==? AND poolType==? AND imsi==? AND type='static'"
	listStaticLeasesByDNSQL     = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE poolID==? AND type='static' ORDER BY poolType, addressBin"
	listStaticLeasesByIMSISQL   = "SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM " + IPLeasesTableName + " WHERE imsi==? AND type='static' ORDER BY addressBin"
	updateStaticLeaseAddressSQL = "UPDATE " + IPLeasesTableName + " SET addressBin=? WHERE id==? AND type='static'"
	deleteStaticLeaseSQL        = "DELETE FROM " + IPLeasesTableName + " WHERE id==? AND type='static'"
)

// IPLease represents a row in the ip_leases table.
type IPLease struct {
	ID         string // UUIDv7
	PoolID     string // FK to data_networks.id (UUID)
	AddressBin []byte
	IMSI       string
	SessionID  *int
	Type       string
	CreatedAt  int64
	NodeID     string
	PoolType   string
}

// Address returns the IP address derived from AddressBin.
// IPv4-mapped IPv6 addresses are unmapped to plain IPv4.
func (l *IPLease) Address() netip.Addr {
	if len(l.AddressBin) != 16 {
		return netip.Addr{}
	}

	var arr [16]byte

	copy(arr[:], l.AddressBin)

	addr := netip.AddrFrom16(arr)
	if addr.Is4In6() {
		addr = addr.Unmap()
	}

	return addr
}

// AllocateIPLease atomically resolves and inserts a dynamic IP lease for
// (poolID, IMSI, sessionID, nodeID). On the leader, the resolution and
// INSERT happen inside leaderCaptureAndPropose's proposeMu; on a
// follower, the call is forwarded to the leader through the existing
// /cluster/internal/propose path, so the allocation is serialised
// regardless of which node initiated it. Returns the chosen address.
func (db *Database) AllocateIPLease(ctx context.Context, poolID string, poolType string, imsi string, sessionID int, nodeID string) (_ netip.Addr, err error) {
	ctx, finish := startDBQueryNamed(ctx, "INSERT "+IPLeasesTableName+" (allocate)", IPLeasesTableName, "INSERT")
	defer func() { finish(err) }()

	addrStr, err := opAllocateIPLease.Invoke(ctx, db, &allocateIPLeasePayload{
		PoolID:    poolID,
		PoolType:  poolType,
		IMSI:      imsi,
		SessionID: sessionID,
		NodeID:    pki.NodeID(nodeID),
	})
	if err != nil {
		return netip.Addr{}, err
	}

	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse allocated address %q: %w", addrStr, err)
	}

	trace.SpanFromContext(ctx).SetAttributes(attribute.String("ip_lease.ipv4", addr.String()))

	return addr, nil
}

// AllocateIPv6Lease atomically resolves and inserts a dynamic IPv6 lease for
// (poolID, poolType, IMSI, sessionID, nodeID). Like AllocateIPLease, it
// forwards intent to the leader through the replication mechanism so that
// concurrent allocations from any node serialise correctly under proposeMu.
// Each allocated unit is a /64 prefix delegated from the IPv6 CIDR pool.
func (db *Database) AllocateIPv6Lease(ctx context.Context, poolID string, poolType string, imsi string, sessionID int, nodeID string) (_ netip.Addr, err error) {
	ctx, finish := startDBQueryNamed(ctx, "INSERT "+IPLeasesTableName+" (allocate_ipv6)", IPLeasesTableName, "INSERT")
	defer func() { finish(err) }()

	addrStr, err := opAllocateIPv6Lease.Invoke(ctx, db, &allocateIPLeasePayload{
		PoolID:    poolID,
		PoolType:  poolType,
		IMSI:      imsi,
		SessionID: sessionID,
		NodeID:    pki.NodeID(nodeID),
	})
	if err != nil {
		return netip.Addr{}, err
	}

	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse allocated address %q: %w", addrStr, err)
	}

	trace.SpanFromContext(ctx).SetAttributes(attribute.String("ip_lease.ipv6", addr.String()))

	return addr, nil
}

// CreateLease inserts a new IP lease row. The address is stored as a 16-byte
// binary form. Returns ErrAlreadyExists if the (poolID, addressBin) unique
// constraint is violated.
func (db *Database) CreateLease(ctx context.Context, lease *IPLease, address netip.Addr) (err error) {
	ctx, finish := startDBQuery(ctx, IPLeasesTableName, "INSERT")
	defer func() { finish(err) }()

	b := address.As16()
	lease.AddressBin = b[:]

	_, err = opCreateLease.Invoke(ctx, db, lease)
	if err != nil {
		return err
	}

	return nil
}

// GetLeaseBySession returns the lease matching (poolID, poolType, sessionID, imsi), or ErrNotFound.
func (db *Database) GetLeaseBySession(ctx context.Context, poolID string, poolType string, sessionID int, imsi string) (_ *IPLease, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (by session)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanIPLease(db.reader().QueryRowContext(ctx, getLeaseBySessionSQL, poolID, poolType, sessionID, imsi))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) GetActiveLeaseByAddress(ctx context.Context, poolID string, address netip.Addr) (_ *IPLease, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (by address)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	b := address.As16()

	row, err := scanIPLease(db.reader().QueryRowContext(ctx, getActiveLeaseByAddressSQL, poolID, b[:]))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

// UpdateLeaseSession sets the sessionID on an existing lease.
func (db *Database) UpdateLeaseSession(ctx context.Context, leaseID string, sessionID int) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+IPLeasesTableName+" (session)", IPLeasesTableName, "UPDATE")
	defer func() { finish(err) }()

	lease := &IPLease{ID: leaseID, SessionID: &sessionID}

	_, err = opUpdateLeaseSession.Invoke(ctx, db, lease)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) ReleaseIPLease(ctx context.Context, poolID string, poolType string, imsi string, sessionID int, nodeID string) (_ netip.Addr, err error) {
	ctx, finish := startDBQueryNamed(ctx, "DELETE "+IPLeasesTableName+" (release)", IPLeasesTableName, "DELETE")
	defer func() { finish(err) }()

	addrStr, err := opReleaseIPLease.Invoke(ctx, db, &releaseIPLeasePayload{
		PoolID:    poolID,
		PoolType:  poolType,
		IMSI:      imsi,
		SessionID: sessionID,
		NodeID:    pki.NodeID(nodeID),
	})
	if err != nil {
		return netip.Addr{}, err
	}

	if addrStr == "" {
		return netip.Addr{}, nil
	}

	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse released address %q: %w", addrStr, err)
	}

	trace.SpanFromContext(ctx).SetAttributes(attribute.String("ip_lease.ipv4", addr.String()))

	return addr, nil
}

func (db *Database) DeleteDynamicLease(ctx context.Context, leaseID string) (err error) {
	ctx, finish := startDBQuery(ctx, IPLeasesTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteDynamicLease.Invoke(ctx, db, &stringPayload{Value: leaseID})
	if err != nil {
		return err
	}

	return nil
}

// DeleteAllDynamicLeases removes all dynamic leases cluster-wide,
// preserving static ones. No production caller.
func (db *Database) DeleteAllDynamicLeases(ctx context.Context) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "DELETE "+IPLeasesTableName+" (dynamic)", IPLeasesTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteAllDynamicLeases.Invoke(ctx, db, nil)
	if err != nil {
		return err
	}

	return nil
}

// DeleteDynamicLeasesByNode removes dynamic leases tagged with the
// given nodeID. Static leases are preserved: an admin-pinned IP stays
// bound to its IMSI regardless of which node previously served it.
func (db *Database) DeleteDynamicLeasesByNode(ctx context.Context, nodeID string) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "DELETE "+IPLeasesTableName+" (dynamic by node)", IPLeasesTableName, "DELETE", attrs.NodeID(nodeID))
	defer func() { finish(err) }()

	_, err = opDeleteDynamicLeasesByNode.Invoke(ctx, db, &nodeIDPayload{Value: pki.NodeID(nodeID)})
	if err != nil {
		return err
	}

	return nil
}

// UpdateLeaseNode updates the nodeID and sessionID on an existing lease.
// Used during failover to transfer lease ownership to the new serving node.
func (db *Database) UpdateLeaseNode(ctx context.Context, leaseID string, nodeID string, sessionID int) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+IPLeasesTableName+" (node)", IPLeasesTableName, "UPDATE")
	defer func() { finish(err) }()

	lease := &IPLease{ID: leaseID, NodeID: nodeID, SessionID: &sessionID}

	_, err = opUpdateLeaseNode.Invoke(ctx, db, lease)
	if err != nil {
		return err
	}

	return nil
}

// ListActiveLeases returns all leases with a non-NULL sessionID (dynamic + active static).
func (db *Database) ListActiveLeases(ctx context.Context) (_ []IPLease, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (active)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	leases, err := queryAll(ctx, db.reader(), listActiveLeasesSQL, scanIPLease)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return leases, nil
}

// ListActiveLeasesByNode returns all active leases (sessionID non-NULL)
// owned by the given cluster node. Used by the BGP reconciler on each
// node to derive its local advertisement set: BGP routes are advertised
// from the node that hosts the PDU session, not by every node in the
// cluster.
func (db *Database) ListActiveLeasesByNode(ctx context.Context, nodeID string) (_ []IPLease, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (active,by_node)", IPLeasesTableName, "SELECT", attrs.NodeID(nodeID))
	defer func() { finish(err) }()

	leases, err := queryAll(ctx, db.reader(), listActiveLeasesByNodeSQL, scanIPLease, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return leases, nil
}

// listAllLeases returns every lease row. Used only by the support bundle export.
func (db *Database) listAllLeases(ctx context.Context) ([]IPLease, error) {
	leases, err := queryAll(ctx, db.reader(), listAllLeasesSQL, scanIPLease)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return leases, nil
}

// ListLeasesByPool returns all leases (dynamic + static) for a given pool.
func (db *Database) ListLeasesByPool(ctx context.Context, poolID string, poolType string) (_ []IPLease, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (by pool)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	leases, err := queryAll(ctx, db.reader(), listLeasesByPoolSQL, scanIPLease, poolID, poolType)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return leases, nil
}

// ListLeasesByPoolPage returns a page of leases for a pool, ordered by address,
// along with the total count. The page parameter is 1-based.
func (db *Database) ListLeasesByPoolPage(ctx context.Context, poolID, poolType string, page, perPage int) (_ []IPLease, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (paged by pool)", IPLeasesTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	leases, err := queryAll(ctx, db.reader(), listLeasesByPoolPageSQL, scanIPLease, poolID, poolType, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count, err := db.CountLeasesByPool(ctx, poolID, poolType)
	if err != nil {
		return nil, 0, fmt.Errorf("count failed: %w", err)
	}

	return leases, count, nil
}

// CountLeasesByPool returns the total number of leases in a pool.
func (db *Database) CountLeasesByPool(ctx context.Context, poolID, poolType string) (_ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (count by pool)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countLeasesByPoolSQL, poolID, poolType)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

// CountIPv4LeasesByPool returns the number of IPv4 leases in a pool.
// IPv4 addresses are stored as IPv4-mapped IPv6 (::ffff:x.x.x.x).
func (db *Database) CountIPv4LeasesByPool(ctx context.Context, poolID, poolType string) (_ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (count IPv4 by pool)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countIPv4LeasesByPoolSQL, poolID, poolType)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

// CountIPv6LeasesByPool returns the number of IPv6 leases in a pool.
func (db *Database) CountIPv6LeasesByPool(ctx context.Context, poolID, poolType string) (_ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (count IPv6 by pool)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countIPv6LeasesByPoolSQL, poolID, poolType)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

// CountActiveLeases returns the total number of active leases (sessionID IS NOT NULL).
func (db *Database) CountActiveLeases(ctx context.Context) (_ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (count active)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countActiveLeasesSQL)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

// CountLeasesByIMSI returns the total number of leases (all types) for a subscriber.
func (db *Database) CountLeasesByIMSI(ctx context.Context, imsi string) (_ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (count by imsi)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countLeasesByIMSISQL, imsi)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

// CreateStaticLease pins an address to a subscriber for a data network and
// family. Returns ErrAlreadyExists when a static lease already exists for
// (poolID, poolType, imsi) or the address is already leased.
func (db *Database) CreateStaticLease(ctx context.Context, imsi, poolID, poolType string, addr netip.Addr) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "INSERT "+IPLeasesTableName+" (static)", IPLeasesTableName, "INSERT")
	defer func() { finish(err) }()

	b := addr.As16()
	lease := &IPLease{
		PoolID:     poolID,
		PoolType:   poolType,
		AddressBin: b[:],
		IMSI:       imsi,
	}

	_, err = opCreateStaticLease.Invoke(ctx, db, lease)
	if err != nil {
		return err
	}

	return nil
}

// GetStaticLease returns the static reservation for (poolID, poolType,
// imsi), or ErrNotFound.
func (db *Database) GetStaticLease(ctx context.Context, poolID, poolType, imsi string) (_ *IPLease, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (static)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanIPLease(db.reader().QueryRowContext(ctx, getStaticLeaseSQL, poolID, poolType, imsi))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

// ListStaticLeasesByIMSI returns every static reservation held by a
// subscriber across data networks and families.
func (db *Database) ListStaticLeasesByIMSI(ctx context.Context, imsi string) (_ []IPLease, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (static by imsi)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	leases, err := queryAll(ctx, db.reader(), listStaticLeasesByIMSISQL, scanIPLease, imsi)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return leases, nil
}

// ListStaticLeasesByDataNetwork returns every static reservation in a pool
// (both families), ordered by family then address.
func (db *Database) ListStaticLeasesByDataNetwork(ctx context.Context, poolID string) (_ []IPLease, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+IPLeasesTableName+" (static by data network)", IPLeasesTableName, "SELECT")
	defer func() { finish(err) }()

	leases, err := queryAll(ctx, db.reader(), listStaticLeasesByDNSQL, scanIPLease, poolID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return leases, nil
}

// ClearStaticLeaseSession returns a static lease to reserved by nulling its
// sessionID, so listActiveLeases and BGP (sessionID IS NOT NULL) drop the
// address while the row persists.
func (db *Database) ClearStaticLeaseSession(ctx context.Context, leaseID string) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+IPLeasesTableName+" (static clear)", IPLeasesTableName, "UPDATE")
	defer func() { finish(err) }()

	_, err = opUpdateLeaseSession.Invoke(ctx, db, &IPLease{ID: leaseID})
	if err != nil {
		return err
	}

	return nil
}

// UpdateStaticLeaseAddress repins a reserved static lease to a new address.
// Returns ErrNotFound if the reservation is gone, ErrAlreadyExists if the
// address is already leased.
func (db *Database) UpdateStaticLeaseAddress(ctx context.Context, leaseID string, addr netip.Addr) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+IPLeasesTableName+" (static repin)", IPLeasesTableName, "UPDATE")
	defer func() { finish(err) }()

	b := addr.As16()

	_, err = opUpdateStaticLeaseAddress.Invoke(ctx, db, &IPLease{ID: leaseID, AddressBin: b[:]})
	if err != nil {
		return err
	}

	return nil
}

// DeleteStaticLease removes a reserved static lease. Returns ErrNotFound if
// the reservation is gone.
func (db *Database) DeleteStaticLease(ctx context.Context, leaseID string) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "DELETE "+IPLeasesTableName+" (static)", IPLeasesTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteStaticLease.Invoke(ctx, db, &stringPayload{Value: leaseID})
	if err != nil {
		return err
	}

	return nil
}

func scanIPLease(row scanner) (IPLease, error) {
	var v IPLease

	if err := row.Scan(&v.ID, &v.PoolID, &v.AddressBin, &v.IMSI, &v.SessionID, &v.Type, &v.CreatedAt, &v.NodeID, &v.PoolType); err != nil {
		return IPLease{}, err
	}

	return v, nil
}

func scanLeaseAddress(row scanner) (IPLease, error) {
	var v IPLease

	if err := row.Scan(&v.AddressBin); err != nil {
		return IPLease{}, err
	}

	return v, nil
}
