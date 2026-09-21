// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
export type NodeId = string | number;

export const nodeIdKey = (id: NodeId | undefined): string =>
  id === undefined ? "" : String(id);

export const sameNodeId = (
  a: NodeId | undefined,
  b: NodeId | undefined,
): boolean => a !== undefined && b !== undefined && String(a) === String(b);

export const shortNodeId = (id: NodeId | undefined): string => {
  const s = nodeIdKey(id);

  return s.length > 12 ? s.slice(-12) : s;
};

export const nodeLabel = (
  displayName: string | undefined,
  id: NodeId | undefined,
): string =>
  displayName && displayName.length > 0 ? displayName : shortNodeId(id);
