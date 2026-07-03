// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

// Radio Access Technology a cell position applies to. Determines whether
// cell_identity is interpreted as an NR Cell Identity (36-bit) or an
// E-UTRA Cell Identity (28-bit).
export type CellPositionRAT = "nr" | "eutra";

export type CellPosition = {
  id: string;
  rat: CellPositionRAT;
  mcc: string;
  mnc: string;
  cell_identity: string;
  gnb_id?: string;
  latitude: number;
  longitude: number;
  altitude?: number;
  uncertainty_semi_major?: number;
  uncertainty_semi_minor?: number;
  orientation_major?: number;
  confidence?: number;
  source: string;
};

export type CellPositionParams = {
  rat: CellPositionRAT;
  mcc: string;
  mnc: string;
  cell_identity: string;
  gnb_id?: string;
  latitude: number;
  longitude: number;
  altitude?: number;
  uncertainty_semi_major?: number;
  uncertainty_semi_minor?: number;
  orientation_major?: number;
  confidence?: number;
};

export async function listCellPositions(
  authToken: string,
): Promise<CellPosition[]> {
  const result = await apiFetch<CellPosition[]>("/api/beta/cell-positions", {
    authToken,
  });
  return result ?? [];
}

export async function getCellPosition(
  authToken: string,
  id: string,
): Promise<CellPosition> {
  return apiFetch<CellPosition>(
    `/api/beta/cell-positions/${encodeURIComponent(id)}`,
    { authToken },
  );
}

export async function createCellPosition(
  authToken: string,
  params: CellPositionParams,
): Promise<void> {
  await apiFetchVoid("/api/beta/cell-positions", {
    method: "POST",
    authToken,
    body: params,
  });
}

export async function updateCellPosition(
  authToken: string,
  id: string,
  params: CellPositionParams,
): Promise<void> {
  await apiFetchVoid(`/api/beta/cell-positions/${encodeURIComponent(id)}`, {
    method: "PUT",
    authToken,
    body: params,
  });
}

export async function deleteCellPosition(
  authToken: string,
  id: string,
): Promise<void> {
  await apiFetchVoid(`/api/beta/cell-positions/${encodeURIComponent(id)}`, {
    method: "DELETE",
    authToken,
  });
}
