// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const TAC_HEX_PATTERN = /^[0-9A-Fa-f]{6}$/;

export const tacToDecimal = (tac: string): number | null => {
  const trimmed = tac.trim();

  if (!/^[0-9A-Fa-f]{1,6}$/.test(trimmed)) return null;

  return parseInt(trimmed, 16);
};

export const formatTacDecimal = (tac: string): string | null => {
  const value = tacToDecimal(tac);

  return value === null ? null : String(value);
};
