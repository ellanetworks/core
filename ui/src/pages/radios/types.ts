// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useOutletContext } from "react-router-dom";

export interface RadiosTabProps {}

export function useRadiosContext() {
  return useOutletContext<RadiosTabProps>();
}
