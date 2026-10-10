// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/udm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/hss")

var (
	ErrSubscriberUnknown = errors.New("subscriber unknown")
	ErrUnavailable       = errors.New("the HSS cannot commit its state")
)

type Subscriber struct {
	IMSI           string
	MSISDN         string
	IMSDataNetwork bool
}

type Store interface {
	RegistrationStore
	IMSDomain(ctx context.Context) (string, error)
	PCSCFReachable(ctx context.Context) (bool, error)
	Subscriber(ctx context.Context, imsi string) (*Subscriber, error)
	SubscriberByMSISDN(ctx context.Context, msisdn string) (*Subscriber, error)
	CountRegistered(ctx context.Context) (int, error)
}

type Credentials interface {
	GenerateIMSVector(ctx context.Context, imsi string, resync *udm.IMSResync) (*udm.IMSAV, error)
}

type Registrar interface {
	Handle(applicationID, commandCode uint32, h diameter.Handler)
}

type HSS struct {
	store       Store
	credentials Credentials
	peers       Peers
	log         *zap.Logger
	now         func() time.Time
}

func New(store Store, credentials Credentials, peers Peers, log *zap.Logger) *HSS {
	return &HSS{store: store, credentials: credentials, peers: peers, log: log, now: time.Now}
}

func (h *HSS) Register(r Registrar) {
	r.Handle(cx.ApplicationID, cx.CommandUserAuthorization, observed("cx/user-authorization", h.UserAuthorization))
	r.Handle(cx.ApplicationID, cx.CommandMultimediaAuth, observed("cx/multimedia-auth", h.MultimediaAuth))
	r.Handle(cx.ApplicationID, cx.CommandServerAssignment, observed("cx/server-assignment", h.ServerAssignment))
	r.Handle(cx.ApplicationID, cx.CommandLocationInfo, observed("cx/location-info", h.LocationInfo))
}

func (h *HSS) errorAnswer(req *diameter.Message, id diameter.Identity, procedure string, err error) *diameter.Message {
	if errors.Is(err, ErrSubscriberUnknown) || errors.Is(err, udm.ErrSubscriberUnknown) {
		return experimentalAnswer(req, id, tgpp.ResultErrorUserUnknown)
	}

	if _, ok := tgpp.ResultOf(err); ok {
		return cx.NewErrorAnswer(req, id, err, 0)
	}

	if errors.Is(err, ErrUnavailable) {
		h.log.Info("Cx "+procedure+" answered as unavailable", zap.Error(err))

		return cx.NewAnswer(req, id, tgpp.Result{Code: diameter.UnavailableResult(req)}, 0)
	}

	h.log.Warn("Cx "+procedure+" failed", zap.Error(err))

	return cx.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply}, 0)
}

func experimentalAnswer(req *diameter.Message, id diameter.Identity, code uint32) *diameter.Message {
	return cx.NewAnswer(req, id, tgpp.Experimental(code), 0)
}

func observed(name string, h func(context.Context, diameter.Identity, *diameter.Message) *diameter.Message) diameter.Handler {
	return diameter.HandlerFunc(func(ctx context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		ctx, span := tracer.Start(ctx, name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("network.protocol.name", "diameter"),
				attribute.Int64("diameter.application_id", int64(req.ApplicationID)),
				attribute.Int64("diameter.command_code", int64(req.CommandCode)),
			),
		)
		defer span.End()

		ans := h(ctx, c.LocalIdentity(), req)
		if ans == nil {
			return nil
		}

		if r, err := tgpp.ParseResult(ans); err == nil {
			span.SetAttributes(attribute.Int64("diameter.result_code", int64(r.Code)))
		}

		return ans
	})
}
