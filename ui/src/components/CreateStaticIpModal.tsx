// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Autocomplete,
  Button,
  Alert,
  Collapse,
} from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import ErrorAlert from "@/components/ErrorAlert";
import { useAuth } from "@/contexts/AuthContext";
import {
  createStaticIp,
  updateStaticIp,
  listEligibleSubscribers,
} from "@/queries/data_networks";

interface StaticIpEdit {
  imsi: string;
  ipVersion: string;
  address: string;
  /** The reservation is bound to a live session, which a change will release. */
  active?: boolean;
}

interface CreateStaticIpModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  dataNetwork: string;
  ipv4Pool?: string;
  ipv6Pool?: string;
  edit?: StaticIpEdit;
}

const CreateStaticIpModal: React.FC<CreateStaticIpModalProps> = ({
  open,
  onClose,
  onSuccess,
  dataNetwork,
  ipv4Pool,
  ipv6Pool,
  edit,
}) => {
  const { accessToken, authReady } = useAuth();
  const isEdit = !!edit;

  const [imsi, setImsi] = useState<string>(edit?.imsi ?? "");
  const [address, setAddress] = useState<string>(edit?.address ?? "");
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");

  useEffect(() => {
    if (open) {
      setImsi(edit?.imsi ?? "");
      setAddress(edit?.address ?? "");
      setAlert("");
    }
  }, [open, edit]);

  const subscribersQuery = useQuery({
    queryKey: ["eligible-subscribers", dataNetwork],
    queryFn: () => listEligibleSubscribers(accessToken!, dataNetwork),
    enabled: open && !isEdit && authReady && !!accessToken,
  });

  const subscribers = subscribersQuery.data;

  const poolHelp = isEdit
    ? edit?.ipVersion === "ipv6"
      ? `IPv6 pool: ${ipv6Pool ?? "—"}`
      : `IPv4 pool: ${ipv4Pool ?? "—"}`
    : [
        ipv4Pool && `IPv4 pool: ${ipv4Pool}`,
        ipv6Pool && `IPv6 pool: ${ipv6Pool}`,
      ]
        .filter(Boolean)
        .join(" · ");

  const canSubmit = imsi.trim() !== "" && address.trim() !== "" && !loading;

  const handleSubmit = async () => {
    if (!accessToken) return;

    setLoading(true);
    setAlert("");

    try {
      if (isEdit && edit) {
        await updateStaticIp(
          accessToken,
          dataNetwork,
          edit.imsi,
          edit.ipVersion,
          address.trim(),
        );
      } else {
        await createStaticIp(
          accessToken,
          dataNetwork,
          imsi.trim(),
          address.trim(),
        );
      }

      onClose();
      onSuccess();
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{isEdit ? "Edit Static IP" : "Add Static IP"}</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert}>
          <Alert onClose={() => setAlert("")} sx={{ mb: 2 }} severity="error">
            {alert}
          </Alert>
        </Collapse>
        {isEdit && edit?.active && (
          <Alert severity="warning" sx={{ mb: 2 }}>
            This subscriber has an active session. Saving a new address releases
            it, and the subscriber reconnects on the new address.
          </Alert>
        )}
        {isEdit ? (
          <TextField
            fullWidth
            label="Subscriber"
            value={imsi}
            margin="normal"
            disabled
          />
        ) : (
          <>
            <Autocomplete
              options={(subscribers ?? []).map((s) => s.imsi)}
              value={imsi || null}
              onChange={(_, value) => setImsi(value ?? "")}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label="Subscriber"
                  margin="normal"
                  autoFocus
                />
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
        )}
        <TextField
          fullWidth
          label="Address"
          value={address}
          onChange={(e) => setAddress(e.target.value)}
          margin="normal"
          helperText={poolHelp}
          placeholder="e.g., 10.45.0.10 or 2001:db8:1::"
          autoFocus={isEdit}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!canSubmit}
        >
          {loading ? "Saving..." : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CreateStaticIpModal;
