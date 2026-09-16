// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

func parseTimeParam(q url.Values, name string, def time.Time) (time.Time, error) {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return def, nil
	}

	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid %s timestamp: must be RFC3339, e.g. 2006-01-02T15:04:05Z", name)
}

func parseTimeRange(q url.Values, defStart time.Time, defEnd time.Time) (time.Time, time.Time, error) {
	start, err := parseTimeParam(q, "start", defStart)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	end, err := parseTimeParam(q, "end", defEnd)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end timestamp must not be before start timestamp")
	}

	return start, end, nil
}
