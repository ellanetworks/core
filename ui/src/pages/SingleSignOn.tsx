// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import {
  Alert,
  Box,
  Button,
  FormControlLabel,
  IconButton,
  Link,
  Skeleton,
  Stack,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  ContentCopy as ContentCopyIcon,
  OpenInNew as OpenInNewIcon,
} from "@mui/icons-material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import ConfirmDialog from "@/components/form/ConfirmDialog";
import EditOIDCProviderModal from "@/components/EditOIDCProviderModal";
import EditOIDCGroupsModal from "@/components/EditOIDCGroupsModal";
import PageTitle from "@/components/PageTitle";
import {
  currentUpdate,
  getOIDC,
  OIDC_QUERY_KEY,
  updateOIDC,
  type OIDCSettings,
} from "@/queries/oidc";
import { roleIDToLabel } from "@/queries/users";
import { describeDuration } from "@/utils/durations";
import {
  MAX_WIDTH,
  PAGE_PADDING_X,
  SETTING_CELL_SX,
  TABLE_CONTAINER_SX,
} from "@/utils/layout";
import { PRODUCT } from "@/utils/product";

const orNone = (value: string) => value || "None";

const missingToEnable = (settings: OIDCSettings) =>
  (
    [
      [settings.external_url, "external URL"],
      [settings.issuer, "issuer URL"],
      [settings.client_id, "client ID"],
      [settings.client_secret_set, "client secret"],
      [settings.display_name, "display name"],
    ] as const
  )
    .filter(([value]) => !value)
    .map(([, label]) => label);

interface SettingRow {
  label: string;
  help: string;
  value: React.ReactNode;
}

function SettingsTable({
  label,
  loading,
  rows,
}: {
  label: string;
  loading: boolean;
  rows: SettingRow[];
}) {
  return (
    <TableContainer sx={TABLE_CONTAINER_SX}>
      <Table aria-label={label}>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.label}>
              <TableCell component="th" scope="row" sx={SETTING_CELL_SX}>
                <Tooltip title={row.help} arrow>
                  <span>{row.label}</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={{ wordBreak: "break-word" }}>
                {loading ? <Skeleton variant="text" width={160} /> : row.value}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function SectionHeader({
  id,
  title,
  action,
}: {
  id: string;
  title: string;
  action: React.ReactNode;
}) {
  return (
    <Stack
      direction="row"
      sx={{
        mb: 2,
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
      }}
    >
      <Typography id={id} variant="h6" component="h2">
        {title}
      </Typography>
      {action}
    </Stack>
  );
}

export default function SingleSignOn() {
  const { accessToken, authReady } = useAuth();
  const queryClient = useQueryClient();
  const { showSnackbar } = useSnackbar();
  const copy = useCopyToClipboard();
  const [editing, setEditing] = useState<
    "provider" | "groups" | "disable" | null
  >(null);
  const [toggling, setToggling] = useState(false);

  const oidcQuery = useQuery<OIDCSettings>({
    queryKey: OIDC_QUERY_KEY,
    queryFn: () => getOIDC(accessToken!),
    enabled: authReady && !!accessToken,
  });
  const data = oidcQuery.data;
  const loading = oidcQuery.isLoading;

  const toggle = async (enabled: boolean) => {
    if (!data || !accessToken) return;
    setToggling(true);
    try {
      const updated = await updateOIDC(accessToken, {
        ...currentUpdate(data),
        enabled,
      });
      queryClient.setQueryData(OIDC_QUERY_KEY, updated);
      void queryClient.invalidateQueries({ queryKey: ["users"] });
      void queryClient.invalidateQueries({ queryKey: ["user"] });
      setEditing(null);
      showSnackbar(
        updated.enabled
          ? "Single sign-on enabled successfully."
          : "Single sign-on disabled successfully.",
        "success",
      );
    } catch (error) {
      setEditing(null);
      showSnackbar(
        `Failed to ${enabled ? "enable" : "disable"} single sign-on: ${
          error instanceof Error ? error.message : "Unknown error"
        }`,
        "error",
      );
    } finally {
      setToggling(false);
    }
  };

  const missing = data ? missingToEnable(data) : [];

  const enabledSwitch = () => {
    if (!data) return <Skeleton variant="text" width={160} />;
    const blocked = !data.enabled && missing.length > 0;
    return (
      <Tooltip
        describeChild
        title={blocked ? `Missing: ${missing.join(", ")}.` : ""}
      >
        <FormControlLabel
          control={
            <Switch
              checked={data.enabled}
              disabled={blocked || toggling}
              onChange={(e) =>
                e.target.checked ? void toggle(true) : setEditing("disable")
              }
            />
          }
          label="Enabled"
        />
      </Tooltip>
    );
  };

  const editButton = (section: "provider" | "groups", label: string) => (
    <Button
      variant="contained"
      color="success"
      size="small"
      disabled={!data}
      aria-label={label}
      onClick={() => setEditing(section)}
    >
      Edit
    </Button>
  );

  const copyableURL = (url: string, label: string) => (
    <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
      <Typography
        variant="body2"
        sx={{ fontFamily: "monospace", overflowWrap: "anywhere" }}
      >
        {url || "Set the external URL first."}
      </Typography>
      {url && (
        <Tooltip title="Copy">
          <IconButton
            size="small"
            aria-label={`Copy the ${label.charAt(0).toLowerCase()}${label.slice(1)}`}
            onClick={() => void copy(url, label)}
          >
            <ContentCopyIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      )}
    </Stack>
  );

  const onSaved = () =>
    showSnackbar("Single sign-on updated successfully.", "success");

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <PageTitle title="Single Sign-On" sx={{ mb: 1 }} />
      <Typography variant="body1" color="textSecondary" sx={{ mb: 3 }}>
        Let users sign in through your organization&apos;s identity provider,
        with OpenID Connect.{" "}
        <Link
          href={PRODUCT.ssoDocsUrl}
          target="_blank"
          rel="noreferrer"
          sx={{ color: "link", whiteSpace: "nowrap" }}
        >
          Configure Single Sign-On
          <OpenInNewIcon
            sx={{ fontSize: 14, ml: 0.5, verticalAlign: "-2px" }}
          />
        </Link>
      </Typography>

      {oidcQuery.isError && (
        <Alert severity="error" sx={{ mb: 3 }}>
          Failed to load the single sign-on settings.
        </Alert>
      )}

      <Box sx={{ mb: 3 }}>{enabledSwitch()}</Box>

      <Box component="section" aria-labelledby="sso-provider-title">
        <SectionHeader
          id="sso-provider-title"
          title="Provider"
          action={editButton("provider", "Edit provider")}
        />
        <SettingsTable
          label="Provider"
          loading={loading}
          rows={[
            {
              label: "Display name",
              help: "The name on the login page's button.",
              value: data && orNone(data.display_name),
            },
            {
              label: "Issuer URL",
              help: "The identity provider.",
              value: data && orNone(data.issuer),
            },
            {
              label: "Client ID",
              help: `The client registered for ${PRODUCT.name} at the identity provider.`,
              value: data && orNone(data.client_id),
            },
            {
              label: "Client secret",
              help: "Never shown.",
              value: data && (data.client_secret_set ? "Set" : "Not set"),
            },
            {
              label: "Redirect URL",
              help: "Register this URL for the client at the identity provider.",
              value: data && copyableURL(data.redirect_url, "Redirect URL"),
            },
            {
              label: "Post-logout redirect URL",
              help: "Register this URL for the client at the identity provider.",
              value:
                data &&
                copyableURL(
                  data.post_logout_redirect_url,
                  "Post-logout redirect URL",
                ),
            },
            {
              label: "Scopes",
              help: `What ${PRODUCT.name} asks the identity provider for.`,
              value: data?.scopes.join(" "),
            },
            {
              label: "Session lifetime",
              help: "After this, users sign in through the identity provider again.",
              value: data && describeDuration(data.session_lifetime_seconds),
            },
            {
              label: "CA certificate",
              help: "Certificates trusted for the identity provider, instead of the system's.",
              value: data && (data.ca_certificate ? "Set" : "System"),
            },
            {
              label: "Trust email addresses",
              help: "Whether email addresses are trusted when the provider does not say they are verified.",
              value: data && (data.trust_unverified_email ? "Yes" : "No"),
            },
          ]}
        />
      </Box>

      <Box
        component="section"
        aria-labelledby="sso-groups-title"
        sx={{ mt: 4 }}
      >
        <SectionHeader
          id="sso-groups-title"
          title="Roles from groups"
          action={editButton("groups", "Edit roles from groups")}
        />
        <SettingsTable
          label="Roles from groups"
          loading={loading}
          rows={[
            {
              label: "Groups claim",
              help: "The claim that holds the user's groups.",
              value: data && orNone(data.groups_claim),
            },
            {
              label: "Groups",
              help: "A user in several mapped groups gets the highest of their roles.",
              value:
                data &&
                (data.group_roles.length === 0 ? (
                  "None"
                ) : (
                  <Stack spacing={0.5}>
                    {data.group_roles.map((g) => (
                      <Typography key={g.group} variant="body2">
                        {g.group} → {roleIDToLabel(g.role_id)}
                      </Typography>
                    ))}
                  </Stack>
                )),
            },
          ]}
        />
      </Box>

      {editing === "provider" && data && (
        <EditOIDCProviderModal
          open
          settings={data}
          onClose={() => setEditing(null)}
          onSuccess={onSaved}
        />
      )}
      {editing === "groups" && data && (
        <EditOIDCGroupsModal
          open
          settings={data}
          onClose={() => setEditing(null)}
          onSuccess={onSaved}
        />
      )}
      <ConfirmDialog
        open={editing === "disable"}
        onClose={() => setEditing(null)}
        onConfirm={() => toggle(false)}
        title="Disable Single Sign-On"
        description="Disable single sign-on? Everyone who logged in with single sign-on is signed out."
        confirmLabel="Disable"
        confirmingLabel="Disabling…"
      />
    </Box>
  );
}
