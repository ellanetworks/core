// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

/**
 * Formats a Date as YYYY-MM-DD in the viewer's local timezone.
 *
 * Deliberately not `toISOString().slice(0, 10)`: that converts to UTC first, so
 * for a viewer east of UTC it reports tomorrow's date, and for one west of UTC
 * it reports tomorrow's date once local time passes the UTC midnight boundary.
 * The date pickers these values feed are local calendar dates, so the defaults
 * have to be local calendar dates too or the two disagree.
 */
export const localDateString = (d: Date): string => {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

/**
 * The inclusive local-date window ending today, `days` days wide, used as the
 * default range on every page that filters by date.
 */
export const defaultDateRange = (
  days = 7,
  now: Date = new Date(),
): { startDate: string; endDate: string } => {
  const start = new Date(now);
  start.setDate(now.getDate() - (days - 1));
  return { startDate: localDateString(start), endDate: localDateString(now) };
};
