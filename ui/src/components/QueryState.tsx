// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Alert, Box, CircularProgress } from "@mui/material";
import type { UseQueryResult } from "@tanstack/react-query";
import ErrorAlert from "@/components/ErrorAlert";

interface QueryStateProps<T> {
  query: UseQueryResult<T>;
  resource: string;
  isEmpty?: (data: T) => boolean;
  empty?: React.ReactNode;
  filtered?: boolean;
  noResults?: React.ReactNode;
  loading?: React.ReactNode;
  children: (
    data: T,
    meta: { stale: boolean; updatedAt: number },
  ) => React.ReactNode;
}

const DefaultLoading: React.FC = () => (
  <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
    <CircularProgress />
  </Box>
);

/**
 * Renders a query's outcome as one of: offline, error, loading, no-results,
 * empty, or data.
 *
 * The branch order is load-bearing:
 *
 * - `isLoadingError` is checked before `isLoading` because a failed first fetch
 *   leaves `isLoading` false (`isPending && isFetching`, and `status` is already
 *   `error`), so an `isLoading` check first would fall through to `empty` and
 *   report a failure as "no data".
 * - `data === undefined` is checked after `isLoading` to cover a query gated by
 *   `enabled: false`, which sits at `isPending` but never `isLoading`.
 * - Zero rows under `filtered` is a no-results state, not an empty one: the
 *   resource may well exist and the filter excluded it, so offering the
 *   empty state's "create one" affordance would be wrong.
 * - `isRefetchError` is passed to `children` as a flag rather than taken as a
 *   branch, so a failed background refresh keeps the last good data on screen.
 */
function QueryState<T>({
  query,
  resource,
  isEmpty,
  empty,
  filtered = false,
  noResults,
  loading,
  children,
}: QueryStateProps<T>) {
  if (query.fetchStatus === "paused") {
    return (
      <Alert severity="warning" sx={{ mt: 2 }}>
        You appear to be offline. {resource} cannot be updated right now.
      </Alert>
    );
  }

  if (query.isLoadingError) {
    return (
      <ErrorAlert
        resource={resource}
        error={query.error}
        onRetry={() => void query.refetch()}
        retrying={query.isFetching}
      />
    );
  }

  if (query.isLoading || query.data === undefined) {
    return <>{loading ?? <DefaultLoading />}</>;
  }

  if (isEmpty?.(query.data)) {
    if (filtered && noResults) return <>{noResults}</>;
    if (!filtered && empty) return <>{empty}</>;
  }

  return (
    <>
      {children(query.data, {
        stale: query.isRefetchError,
        updatedAt: query.dataUpdatedAt,
      })}
    </>
  );
}

export default QueryState;
