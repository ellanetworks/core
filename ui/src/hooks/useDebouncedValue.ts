// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useEffect, useState } from "react";

export const FILTER_DEBOUNCE_MS = 400;

/** Returns `value` once it has stopped changing for `delayMs`. */
export function useDebouncedValue<T>(
  value: T,
  delayMs = FILTER_DEBOUNCE_MS,
): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}
