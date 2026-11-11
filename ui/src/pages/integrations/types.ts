// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useOutletContext } from "react-router-dom";

export interface IntegrationsTabProps {
  canEdit: boolean;
}

export function useIntegrationsContext() {
  return useOutletContext<IntegrationsTabProps>();
}
