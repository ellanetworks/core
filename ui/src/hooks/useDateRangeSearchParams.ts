// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { defaultDateRange } from "@/utils/dates";

const START_PARAM = "start";
const END_PARAM = "end";

export function useDateRangeSearchParams(): {
  startDate: string;
  endDate: string;
  handleStartChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  handleEndChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
} {
  const [searchParams, setSearchParams] = useSearchParams();
  const fallback = useMemo(() => defaultDateRange(), []);

  const setParam = useCallback(
    (key: string, value: string) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set(key, value);
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const handleStartChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) =>
      setParam(START_PARAM, e.target.value),
    [setParam],
  );

  const handleEndChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) =>
      setParam(END_PARAM, e.target.value),
    [setParam],
  );

  return {
    startDate: searchParams.get(START_PARAM) ?? fallback.startDate,
    endDate: searchParams.get(END_PARAM) ?? fallback.endDate,
    handleStartChange,
    handleEndChange,
  };
}
