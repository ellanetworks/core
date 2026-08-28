// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Navigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/contexts/AuthContext";
import QueryState from "@/components/QueryState";
import EmptyState from "@/components/EmptyState";
import { findRadioByName, radioPath } from "@/queries/radios";

const RadioByName: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const { accessToken, authReady } = useAuth();

  const query = useQuery({
    queryKey: ["radio-by-name", name],
    queryFn: () => findRadioByName(accessToken!, name!),
    enabled: authReady && !!accessToken && !!name,
    retry: false,
  });

  return (
    <QueryState
      query={query}
      resource="radio"
      isEmpty={(radio) => radio === null}
      empty={
        <EmptyState
          primaryText={`No radio named "${name}"`}
          secondaryText="It may have been forgotten, or never connected."
        />
      }
    >
      {(radio) =>
        radio && (
          <Navigate
            to={`/radios/${radioPath({ type: radio.type, id: radio.id })}`}
            replace
          />
        )
      }
    </QueryState>
  );
};

export default RadioByName;
