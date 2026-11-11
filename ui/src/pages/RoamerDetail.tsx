// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Skeleton,
  Typography,
} from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getRoamer, type APIRoamer } from "@/queries/roamers";
import { mergeRegistrations } from "@/queries/subscribers";
import { formatPLMN } from "@/queries/roaming";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import DeregisterRoamerModal from "@/components/DeregisterRoamerModal";
import SubscriberConnectionCard from "@/components/SubscriberConnectionCard";
import SubscriberSessionsCard from "@/components/SubscriberSessionsCard";
import QueryState from "@/components/QueryState";
import { formatDateTime } from "@/utils/formatters";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import PageTitle from "@/components/PageTitle";

const RoamerField: React.FC<{ label: string; value?: string }> = ({
  label,
  value,
}) => (
  <Box
    sx={{
      display: "flex",
      alignItems: "center",
      py: 0.75,
      minHeight: 40,
      "&:not(:last-child)": {
        borderBottom: "1px solid",
        borderColor: "divider",
      },
    }}
  >
    <Typography
      variant="body2"
      sx={{ color: "text.secondary", minWidth: 180, flexShrink: 0, mr: 2 }}
    >
      {label}
    </Typography>
    <Typography variant="body2" sx={{ flex: 1, wordBreak: "break-all" }}>
      {value || "—"}
    </Typography>
  </Box>
);

const RoamerDetail: React.FC = () => {
  const { imsi } = useParams<{ imsi: string }>();
  const navigate = useNavigate();
  const { role, accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const canEdit = role === "Admin" || role === "Network Manager";

  const [isDeregisterConfirmOpen, setDeregisterConfirmOpen] = useState(false);

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const roamerQuery = useQuery<APIRoamer>({
    queryKey: ["roamer", imsi],
    queryFn: () => getRoamer(accessToken!, imsi!),
    enabled: authReady && !!accessToken && !!imsi,
    refetchInterval: 5000,
    retry: false,
  });

  const loadingBody = (
    <>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          gap: 3,
        }}
      >
        <Skeleton variant="rounded" height={240} />
        <Skeleton variant="rounded" height={240} />
      </Box>
      <Skeleton variant="rounded" height={240} sx={{ mt: 3 }} />
    </>
  );

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          alignItems: { xs: "flex-start", sm: "center" },
          gap: 2,
          mb: 3,
        }}
      >
        <Box sx={{ flex: 1 }}>
          <PageTitle
            parent={[
              { label: "Subscribers", to: "/subscribers" },
              { label: "Roamers", to: "/subscribers/roamers" },
            ]}
            title={imsi ?? ""}
          />
        </Box>
        {canEdit && roamerQuery.isSuccess && (
          <Button
            variant="outlined"
            color="error"
            onClick={() => setDeregisterConfirmOpen(true)}
          >
            Deregister
          </Button>
        )}
      </Box>

      <QueryState
        query={roamerQuery}
        resource="this roamer"
        loading={loadingBody}
      >
        {(roamer) => (
          <>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
                gap: 3,
                alignItems: "stretch",
              }}
            >
              <Card variant="outlined">
                <CardContent>
                  <Typography variant="h6" sx={{ mb: 1.5 }}>
                    Roamer
                  </Typography>
                  <RoamerField label="IMSI" value={roamer.imsi} />
                  <RoamerField label="Partner" value={roamer.partner} />
                  <RoamerField
                    label="Home PLMN"
                    value={
                      roamer.home_plmn
                        ? formatPLMN(roamer.home_plmn)
                        : undefined
                    }
                  />
                  <RoamerField label="MSISDN" value={roamer.msisdn} />
                  <RoamerField
                    label="Since"
                    value={
                      roamer.since ? formatDateTime(roamer.since) : undefined
                    }
                  />
                </CardContent>
              </Card>

              <SubscriberConnectionCard
                status={mergeRegistrations(roamer.registrations)}
              />
            </Box>

            <SubscriberSessionsCard
              sessions={roamer.sessions}
              loading={roamerQuery.isLoading}
            />
          </>
        )}
      </QueryState>

      {isDeregisterConfirmOpen && imsi && (
        <DeregisterRoamerModal
          open
          onClose={() => setDeregisterConfirmOpen(false)}
          onSuccess={() => {
            showSnackbar(
              `Roamer "${imsi}" deregistered successfully.`,
              "success",
            );
            navigate("/subscribers/roamers");
          }}
          imsi={imsi}
        />
      )}
    </Box>
  );
};

export default RoamerDetail;
