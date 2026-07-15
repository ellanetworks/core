// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type { AlertColor } from "@mui/material";
import { useOutletContext } from "react-router-dom";

export interface NetworkingTabProps {
  accessToken: string | null;
  canEdit: boolean;
  showSnackbar: (message: string, severity: AlertColor) => void;
}

export function useNetworkingContext() {
  return useOutletContext<NetworkingTabProps>();
}
