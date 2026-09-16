// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const startOfLocalDay = (daysBack = 0, now: Date = new Date()): Date => {
  const at = new Date(now);
  at.setDate(at.getDate() - Math.round(daysBack));
  at.setHours(0, 0, 0, 0);
  return at;
};
