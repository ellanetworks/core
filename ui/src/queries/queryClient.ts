// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "@/queries/utils";

/**
 * Retrying a 4xx cannot succeed and only delays the error reaching the user,
 * who waits on the full backoff before any state is rendered.
 */
export const createQueryClient = (): QueryClient =>
  new QueryClient({
    defaultOptions: {
      queries: {
        retry: (failureCount, error) => {
          if (error instanceof ApiError && !error.retryable) return false;
          return failureCount < 2;
        },
      },
    },
  });
