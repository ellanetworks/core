// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import {
  CUSTOM_RANGE,
  toInstant,
  type TimeRangeValue,
} from "@/components/TimeRangePicker";

const START_PARAM = "start";
const END_PARAM = "end";
const RANGE_PARAM = "range";

export function useTimeRangeSearchParams(options: {
  defaultPreset: string;
}): [TimeRangeValue, (next: TimeRangeValue) => void] {
  const { defaultPreset } = options;
  const [searchParams, setSearchParams] = useSearchParams();

  const timeRange = useMemo(() => {
    const from = toInstant(searchParams.get(START_PARAM) ?? "");
    const to = toInstant(searchParams.get(END_PARAM) ?? "");
    const preset =
      searchParams.get(RANGE_PARAM) ??
      (from || to ? CUSTOM_RANGE : defaultPreset);
    if (preset !== CUSTOM_RANGE) return { preset, from: "", to: "" };
    return { preset, from, to };
  }, [searchParams, defaultPreset]);

  const setTimeRange = useCallback(
    (next: TimeRangeValue) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          if (next.preset) params.set(RANGE_PARAM, next.preset);
          else params.delete(RANGE_PARAM);
          if (next.preset === CUSTOM_RANGE) {
            if (next.from) params.set(START_PARAM, next.from);
            else params.delete(START_PARAM);
            if (next.to) params.set(END_PARAM, next.to);
            else params.delete(END_PARAM);
          } else {
            params.delete(START_PARAM);
            params.delete(END_PARAM);
          }
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  return [timeRange, setTimeRange];
}
