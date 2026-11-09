// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type { setupApiServer } from "@/test/apiServer";

export const DIAMETER_STATUS_PATH = "/api/v1/integrations/diameter";

export const diameterStatus = {
  realm: "epc.mnc001.mcc001.3gppnetwork.org",
  servedRealms: [
    "epc.mnc001.mcc001.3gppnetwork.org",
    "ims.mnc001.mcc001.3gppnetwork.org",
  ],
  nodes: [
    {
      nodeId: "1",
      originHost: "mmec01.mmegi8001.mme.epc.mnc001.mcc001.3gppnetwork.org",
      local: true,
    },
  ],
  listener: { enabled: true, addresses: ["10.0.0.2"], port: 3868 },
  peers: [],
};

export const stubDiameterStatus = (
  api: ReturnType<typeof setupApiServer>,
  status: unknown = diameterStatus,
) => api.get(DIAMETER_STATUS_PATH, () => status);
