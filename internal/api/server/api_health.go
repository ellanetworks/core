// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	CheckShutdown    = "shutdown"
	CheckMember      = "member"
	CheckInitialized = "initialized"
	CheckDrain       = "drain"
	CheckDatabase    = "database"
	CheckLeader      = "leader"
	CheckCluster     = "cluster"
)

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type Readiness struct {
	Ready  bool    `json:"ready"`
	Checks []Check `json:"checks"`
}

type readinessProbe struct {
	ctx       context.Context
	db        *db.Database
	discovery bool
	now       func() time.Time
}

func pass(name string) Check {
	return Check{Name: name, OK: true}
}

func fail(name string, detail string) Check {
	return Check{Name: name, Detail: detail}
}

func Livez() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func Readyz(ctx context.Context, dbInstance *db.Database, discovery bool) http.Handler {
	p := readinessProbe{
		ctx:       ctx,
		db:        dbInstance,
		discovery: discovery,
		now:       time.Now,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		withCluster := false

		if v := r.URL.Query().Get("cluster"); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "cluster must be true or false", err, logger.APILog)
				return
			}

			withCluster = b
		}

		res := Readiness{Ready: true, Checks: p.checks(r.Context(), withCluster)}

		for _, c := range res.Checks {
			res.Ready = res.Ready && c.OK
		}

		status := http.StatusOK
		if !res.Ready {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Cache-Control", "no-store")
		writeResponse(r.Context(), w, res, status, logger.APILog)
	})
}

func (p readinessProbe) checks(ctx context.Context, withCluster bool) []Check {
	checks := []Check{
		p.checkShutdown(),
		p.checkMember(),
		p.checkInitialized(ctx),
		p.checkDrain(ctx),
		p.checkDatabase(ctx),
		p.checkLeader(),
	}

	if withCluster {
		checks = append(checks, p.checkCluster(ctx))
	}

	return checks
}

func (p readinessProbe) checkShutdown() Check {
	if p.ctx != nil && p.ctx.Err() != nil {
		return fail(CheckShutdown, "Ella Core is shutting down")
	}

	return pass(CheckShutdown)
}

func (p readinessProbe) checkMember() Check {
	if p.discovery {
		if p.db.ClusterEnabled() {
			return fail(CheckMember, "the cluster is forming")
		}

		return fail(CheckMember, "Ella Core is starting")
	}

	return pass(CheckMember)
}

func (p readinessProbe) checkInitialized(ctx context.Context) Check {
	if !p.db.ClusterEnabled() {
		return pass(CheckInitialized)
	}

	users, err := p.db.CountUsers(ctx)

	switch {
	case err != nil:
		return fail(CheckInitialized, "count the users: "+err.Error())
	case users == 0:
		return fail(CheckInitialized, "not initialized: create the first user on this node, or join it to a cluster")
	}

	return pass(CheckInitialized)
}

func (p readinessProbe) checkDrain(ctx context.Context) Check {
	if !p.db.ClusterEnabled() {
		return pass(CheckDrain)
	}

	m, err := p.db.GetClusterMember(ctx, p.db.RaftID())

	switch {
	case errors.Is(err, db.ErrNotFound):
		return pass(CheckDrain)
	case err != nil:
		return fail(CheckDrain, "read the drain state: "+err.Error())
	case m.DrainState != "" && m.DrainState != db.DrainStateActive:
		return fail(CheckDrain, "the node is "+m.DrainState)
	}

	return pass(CheckDrain)
}

func (p readinessProbe) checkDatabase(ctx context.Context) Check {
	if err := p.db.Ping(ctx); err != nil {
		return fail(CheckDatabase, err.Error())
	}

	return pass(CheckDatabase)
}

func (p readinessProbe) checkLeader() Check {
	if !p.db.HasRaft() || p.db.IsLeader() {
		return pass(CheckLeader)
	}

	last := p.db.RaftLastContact()
	if last.IsZero() {
		return fail(CheckLeader, "no contact with a cluster leader")
	}

	if age := p.now().Sub(last); age > 2*p.db.RaftElectionTimeout() {
		return fail(CheckLeader, fmt.Sprintf("no contact with the cluster leader for %s", age.Round(time.Second)))
	}

	return pass(CheckLeader)
}

func (p readinessProbe) checkCluster(ctx context.Context) Check {
	if !p.db.ClusterEnabled() {
		return pass(CheckCluster)
	}

	if !p.db.IsLeader() {
		return fail(CheckCluster, "only the leader sees the health of the voters")
	}

	health := clusterHealth(ctx, p.db)
	if health.State != ClusterHealthHealthy {
		return fail(CheckCluster, "the cluster is "+health.State)
	}

	return pass(CheckCluster)
}
