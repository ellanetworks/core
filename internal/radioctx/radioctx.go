// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package radioctx

import "context"

type key struct{}

func With(ctx context.Context, radioID string) context.Context {
	if radioID == "" {
		return ctx
	}

	return context.WithValue(ctx, key{}, radioID)
}

func From(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(key{}).(string)

	return id, ok && id != ""
}
