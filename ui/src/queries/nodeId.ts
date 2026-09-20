// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// A node identity is a UUID on a node that joined from this release
// onwards, and the decimal integer a node formed under an earlier
// release keeps. The API sends the legacy form as a JSON number so a
// node running an older binary can still read it.
export type NodeId = string | number;

export const nodeIdKey = (id: NodeId | undefined): string =>
  id === undefined ? "" : String(id);

export const sameNodeId = (
  a: NodeId | undefined,
  b: NodeId | undefined,
): boolean => a !== undefined && b !== undefined && String(a) === String(b);

// shortNodeId trims a UUID to its first group for display. A legacy
// integer is short already and is returned unchanged.
export const shortNodeId = (id: NodeId | undefined): string => {
  const s = nodeIdKey(id);

  return s.length > 12 ? `${s.slice(0, 8)}…` : s;
};

// nodeLabel is the operator-facing handle: the display name where an
// operator set one, and the shortened identity where they did not.
export const nodeLabel = (
  displayName: string | undefined,
  id: NodeId | undefined,
): string =>
  displayName && displayName.length > 0 ? displayName : shortNodeId(id);
