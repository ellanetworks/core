// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useQueries, useQuery } from "@tanstack/react-query";
import { listSlices } from "@/queries/slices";
import { listSliceDataNetworks } from "@/queries/slice_data_networks";

export type SliceDataNetwork = {
  slice: string;
  data_network: string;
  nat: boolean;
  local_switch: boolean;
  network_instance: string;
};

export const sliceDataNetworksKey = ["slice-data-networks"];

export function useSliceDataNetworks(
  accessToken: string | null,
  enabled = true,
) {
  const slicesQuery = useQuery({
    queryKey: [...sliceDataNetworksKey, "slices"],
    queryFn: () => listSlices(accessToken!, 1, 100),
    enabled: enabled && !!accessToken,
  });

  const slices = slicesQuery.data?.items.map((s) => s.name) ?? [];

  const served = useQueries({
    queries: slices.map((slice) => ({
      queryKey: [...sliceDataNetworksKey, slice],
      queryFn: () => listSliceDataNetworks(accessToken!, slice),
      enabled: enabled && !!accessToken,
    })),
  });

  const pairs: SliceDataNetwork[] = served.flatMap((q, i) =>
    (q.data?.items ?? []).map((item) => ({ slice: slices[i], ...item })),
  );

  return {
    slices,
    pairs,
    isLoading: slicesQuery.isLoading || served.some((q) => q.isLoading),
    isError: slicesQuery.isError || served.some((q) => q.isError),
  };
}
