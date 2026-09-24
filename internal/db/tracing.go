// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"
	"strconv"

	"github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

func recordSpanError(span trace.Span, err error) {
	var se sqlite3.Error
	if errors.As(err, &se) {
		code := strconv.Itoa(int(se.ExtendedCode))
		span.SetAttributes(
			semconv.DBResponseStatusCode(code),
			semconv.ErrorTypeKey.String(code),
		)
	} else {
		span.SetAttributes(semconv.ErrorType(err))
	}

	span.SetStatus(codes.Error, err.Error())
}
