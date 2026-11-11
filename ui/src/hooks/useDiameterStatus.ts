// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/contexts/AuthContext";
import { getDiameterStatus, type DiameterStatus } from "@/queries/integrations";

export const DIAMETER_STATUS_KEY = ["integrations-diameter"];

export const useDiameterStatus = (enabled = true, poll = true) => {
  const { accessToken, authReady } = useAuth();

  return useQuery<DiameterStatus>({
    queryKey: DIAMETER_STATUS_KEY,
    enabled: enabled && authReady && !!accessToken,
    refetchInterval: poll ? 5000 : false,
    staleTime: poll ? 0 : Infinity,
    queryFn: () => getDiameterStatus(accessToken!),
    placeholderData: (prev) => prev,
  });
};
