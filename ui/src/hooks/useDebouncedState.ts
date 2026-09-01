// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useCallback, useEffect, useState } from "react";

export const FILTER_DEBOUNCE_MS = 400;

export function useDebouncedState(initial = "", delayMs = FILTER_DEBOUNCE_MS) {
  const [value, setValue] = useState(initial);
  const [applied, setApplied] = useState(initial);

  useEffect(() => {
    if (value === applied) return;
    const timer = setTimeout(() => setApplied(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, applied, delayMs]);

  const applyNow = useCallback((next: string) => {
    setValue(next);
    setApplied(next);
  }, []);

  return [value, applied, setValue, applyNow] as const;
}
