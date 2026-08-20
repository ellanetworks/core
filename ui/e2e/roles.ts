// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { RoleID } from "./api";

export const ROLE_PASSWORD = "E2eRolePassw0rd!";

export const ROLES = {
  "Read Only": {
    email: "e2e-readonly@ellanetworks.com",
    roleId: RoleID.ReadOnly,
    session: "e2e/.auth/readonly.json",
    canEdit: false,
  },
  "Network Manager": {
    email: "e2e-netmanager@ellanetworks.com",
    roleId: RoleID.NetworkManager,
    session: "e2e/.auth/network-manager.json",
    canEdit: true,
  },
} as const;
