// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const localDateString = (d: Date): string => {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

export const defaultDateRange = (
  days = 7,
  now: Date = new Date(),
): { startDate: string; endDate: string } => {
  const start = new Date(now);
  start.setDate(now.getDate() - (days - 1));
  return { startDate: localDateString(start), endDate: localDateString(now) };
};
