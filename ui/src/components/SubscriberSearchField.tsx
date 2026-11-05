// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo } from "react";
import { Autocomplete, TextField } from "@mui/material";
import type { SxProps, TextFieldProps, Theme } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import ErrorAlert from "@/components/ErrorAlert";
import { useAuth } from "@/contexts/AuthContext";
import { useDebouncedState } from "@/hooks/useDebouncedState";
import { searchSubscriberImsis } from "@/queries/subscribers";

interface SubscriberSearchFieldProps {
  value: string;
  onChange: (imsi: string) => void;
  onBlur?: () => void;
  dataNetwork?: string;
  enabled?: boolean;
  label?: string;
  placeholder?: string;
  size?: "small" | "medium";
  margin?: TextFieldProps["margin"];
  autoFocus?: boolean;
  showError?: boolean;
  sx?: SxProps<Theme>;
}

const SubscriberSearchField: React.FC<SubscriberSearchFieldProps> = ({
  value,
  onChange,
  onBlur,
  dataNetwork,
  enabled = true,
  label = "Subscriber",
  placeholder,
  size,
  margin,
  autoFocus,
  showError = false,
  sx,
}) => {
  const { accessToken, authReady } = useAuth();
  const [inputValue, appliedSearch, setInputValue] = useDebouncedState();
  const search = appliedSearch.trim() === value ? "" : appliedSearch.trim();

  const query = useQuery({
    queryKey: ["subscriber-search", dataNetwork ?? "", search],
    queryFn: () => searchSubscriberImsis(accessToken!, search, dataNetwork),
    enabled: enabled && authReady && !!accessToken,
    placeholderData: (prev) => prev,
  });

  const options = useMemo(() => {
    const found = query.data ?? [];
    return value && !found.includes(value) ? [value, ...found] : found;
  }, [query.data, value]);

  return (
    <>
      <Autocomplete
        options={options}
        value={value || null}
        onChange={(_event, next) => onChange(next ?? "")}
        onBlur={onBlur}
        inputValue={inputValue}
        onInputChange={(_event, next) => setInputValue(next)}
        filterOptions={(x) => x}
        loading={query.isFetching}
        noOptionsText="No matching subscribers"
        size={size}
        sx={sx}
        renderInput={(params) => (
          <TextField
            {...params}
            label={label}
            placeholder={placeholder}
            margin={margin}
            autoFocus={autoFocus}
          />
        )}
      />
      {showError && query.isLoadingError && (
        <ErrorAlert
          resource="subscribers"
          error={query.error}
          onRetry={() => void query.refetch()}
          retrying={query.isFetching}
        />
      )}
    </>
  );
};

export default SubscriberSearchField;
