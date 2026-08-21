// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Autocomplete, TextField } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { useController } from "react-hook-form";
import type { Control, FieldValues, Path } from "react-hook-form";
import ErrorAlert from "@/components/ErrorAlert";
import { useAuth } from "@/contexts/AuthContext";
import { listEligibleSubscribers } from "@/queries/data_networks";

interface SubscriberSelectFieldProps<T extends FieldValues> {
  control: Control<T>;
  name: Path<T>;
  dataNetwork: string;
  open: boolean;
  readOnly?: boolean;
}

const SubscriberSelectField = <T extends FieldValues>({
  control,
  name,
  dataNetwork,
  open,
  readOnly = false,
}: SubscriberSelectFieldProps<T>) => {
  const { accessToken, authReady } = useAuth();
  const { field } = useController({ control, name });

  const subscribersQuery = useQuery({
    queryKey: ["eligible-subscribers", dataNetwork],
    queryFn: () => listEligibleSubscribers(accessToken!, dataNetwork),
    enabled: open && !readOnly && authReady && !!accessToken,
  });

  if (readOnly) {
    return (
      <TextField
        fullWidth
        label="Subscriber"
        value={field.value ?? ""}
        margin="normal"
        disabled
      />
    );
  }

  return (
    <>
      <Autocomplete
        options={(subscribersQuery.data ?? []).map((s) => s.imsi)}
        value={field.value || null}
        onChange={(_event, value) => field.onChange(value ?? "")}
        onBlur={field.onBlur}
        renderInput={(params) => (
          <TextField {...params} label="Subscriber" margin="normal" autoFocus />
        )}
      />
      {subscribersQuery.isLoadingError && (
        <ErrorAlert
          resource="eligible subscribers"
          error={subscribersQuery.error}
          onRetry={() => void subscribersQuery.refetch()}
          retrying={subscribersQuery.isFetching}
        />
      )}
    </>
  );
};

export default SubscriberSelectField;
