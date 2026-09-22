// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useQuery } from "@tanstack/react-query";
import { getStatus, type APIStatus } from "@/queries/status";

export const STATUS_QUERY_KEY = ["status"];

export const useStatusQuery = () =>
  useQuery<APIStatus>({
    queryKey: STATUS_QUERY_KEY,
    queryFn: getStatus,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });
