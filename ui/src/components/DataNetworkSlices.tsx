// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import {
  Alert,
  Box,
  Button,
  IconButton,
  MenuItem,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import DeleteIcon from "@mui/icons-material/Delete";
import { useQueryClient } from "@tanstack/react-query";
import {
  createSliceDataNetwork,
  deleteSliceDataNetwork,
  updateSliceDataNetwork,
} from "@/queries/slice_data_networks";
import {
  sliceDataNetworksKey,
  type SliceDataNetwork,
} from "@/hooks/useSliceDataNetworks";
import { PRODUCT } from "@/utils/product";

interface DataNetworkSlicesProps {
  accessToken: string;
  dataNetwork: string;
  ims: boolean;
  slices: string[];
  pairs: SliceDataNetwork[];
  networkInstances: string[];
  canEdit: boolean;
  onError: (message: string) => void;
}

const DataNetworkSlices: React.FC<DataNetworkSlicesProps> = ({
  accessToken,
  dataNetwork,
  ims,
  slices,
  pairs,
  networkInstances,
  canEdit,
  onError,
}) => {
  const queryClient = useQueryClient();
  const served = pairs.filter((p) => p.data_network === dataNetwork);
  const unserved = slices.filter((s) => !served.some((p) => p.slice === s));
  const [newSlice, setNewSlice] = useState("");

  const run = async (action: () => Promise<void>, failure: string) => {
    try {
      await action();
    } catch (error) {
      onError(`${failure}: ${String(error)}`);
    } finally {
      await queryClient.invalidateQueries({ queryKey: sliceDataNetworksKey });
    }
  };

  const serve = (slice: string) =>
    run(
      () => createSliceDataNetwork(accessToken, slice, dataNetwork, !ims, ims),
      `Failed to serve ${dataNetwork} on slice ${slice}`,
    ).then(() => setNewSlice(""));

  const update = (
    slice: string,
    settings: {
      nat?: boolean;
      local_switch?: boolean;
      network_instance?: string;
    },
  ) =>
    run(
      () => updateSliceDataNetwork(accessToken, slice, dataNetwork, settings),
      `Failed to update ${dataNetwork} on slice ${slice}`,
    );

  const remove = (slice: string) =>
    run(
      () => deleteSliceDataNetwork(accessToken, slice, dataNetwork),
      `Failed to stop serving ${dataNetwork} on slice ${slice}`,
    );

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h6" sx={{ mb: 0.5 }}>
        Slices
      </Typography>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 1.5 }}>
        Slices that serve this data network, each with its own forwarding. NAT
        is set on each {PRODUCT.name} node; local switching only connects
        subscribers on the same slice and data network. The network instance
        picks the N6 interface, defined in each node's configuration file; a
        change applies to new sessions.
      </Typography>
      {served.length === 0 && (
        <Alert severity="info" sx={{ mb: 2 }}>
          No slice serves this data network yet, so no policy can use it.
        </Alert>
      )}
      {served.length > 0 && (
        <Table size="small" sx={{ mb: 2 }}>
          <TableHead>
            <TableRow>
              <TableCell>Slice</TableCell>
              <TableCell>NAT</TableCell>
              <TableCell>Local switch</TableCell>
              <TableCell>Network instance</TableCell>
              {canEdit && <TableCell />}
            </TableRow>
          </TableHead>
          <TableBody>
            {served.map((p) => (
              <TableRow key={p.slice}>
                <TableCell>{p.slice}</TableCell>
                <TableCell>
                  <Switch
                    checked={p.nat}
                    disabled={!canEdit || ims}
                    onChange={(_, on) => update(p.slice, { nat: on })}
                    slotProps={{ input: { "aria-label": `NAT on ${p.slice}` } }}
                  />
                </TableCell>
                <TableCell>
                  <Switch
                    checked={p.local_switch}
                    disabled={!canEdit || ims}
                    onChange={(_, on) => update(p.slice, { local_switch: on })}
                    slotProps={{
                      input: { "aria-label": `Local switch on ${p.slice}` },
                    }}
                  />
                </TableCell>
                <TableCell>
                  <TextField
                    select
                    size="small"
                    value={p.network_instance || "default"}
                    disabled={!canEdit}
                    onChange={(e) =>
                      update(p.slice, { network_instance: e.target.value })
                    }
                    slotProps={{
                      htmlInput: {
                        "aria-label": `Network instance on ${p.slice}`,
                      },
                    }}
                    sx={{ minWidth: 140 }}
                  >
                    {[
                      ...new Set([
                        "default",
                        ...networkInstances,
                        p.network_instance || "default",
                      ]),
                    ].map((ni) => (
                      <MenuItem key={ni} value={ni}>
                        {ni}
                      </MenuItem>
                    ))}
                  </TextField>
                </TableCell>
                {canEdit && (
                  <TableCell align="right">
                    <Tooltip title="Stop serving on this slice">
                      <IconButton
                        size="small"
                        aria-label={`Stop serving on ${p.slice}`}
                        onClick={() => remove(p.slice)}
                      >
                        <DeleteIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      {canEdit && unserved.length > 0 && (
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          <TextField
            select
            size="small"
            label="Slice"
            value={newSlice}
            onChange={(e) => setNewSlice(e.target.value)}
            sx={{ minWidth: 200 }}
          >
            {unserved.map((s) => (
              <MenuItem key={s} value={s}>
                {s}
              </MenuItem>
            ))}
          </TextField>
          <Button
            variant="outlined"
            size="small"
            startIcon={<AddIcon />}
            disabled={!newSlice}
            onClick={() => serve(newSlice)}
          >
            Serve on slice
          </Button>
        </Box>
      )}
    </Box>
  );
};

export default DataNetworkSlices;
