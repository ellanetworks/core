// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Box,
  IconButton,
  List,
  ListItem,
  ListItemText,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  ArrowDownward as ArrowDownwardIcon,
  ArrowUpward as ArrowUpwardIcon,
  Delete as DeleteIcon,
} from "@mui/icons-material";
import {
  listIMSServices,
  updateProfileIMSServices,
  type IMSService,
} from "@/queries/ims";
import { useAuth } from "@/contexts/AuthContext";
import ConfirmDialog from "@/components/form/ConfirmDialog";

interface EditProfileIMSServicesModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  profileName: string;
  initialServices: string[];
}

const EditProfileIMSServicesModal: React.FC<
  EditProfileIMSServicesModalProps
> = ({ open, onClose, onSuccess, profileName, initialServices }) => {
  const { accessToken, authReady } = useAuth();
  const [selected, setSelected] = useState<string[]>(initialServices);

  const servicesQuery = useQuery<IMSService[]>({
    queryKey: ["ims-services"],
    enabled: authReady && !!accessToken,
    queryFn: () => listIMSServices(accessToken!),
  });
  const available = (servicesQuery.data ?? []).filter(
    (s) => !selected.includes(s.name),
  );

  const move = (index: number, delta: number) => {
    setSelected((list) => {
      const next = [...list];
      const [item] = next.splice(index, 1);
      next.splice(index + delta, 0, item);
      return next;
    });
  };

  const save = async () => {
    if (!accessToken) return;
    await updateProfileIMSServices(accessToken, profileName, selected);
    onClose();
    onSuccess();
  };

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={save}
      title="Edit IMS Services"
      description="Services earlier in the list get higher iFC priority."
      confirmLabel="Save"
      confirmingLabel="Saving…"
      confirmColor="success"
      fullWidth
      extra={
        <Box sx={{ mt: 2 }}>
          {selected.length === 0 ? (
            <Typography variant="body2" color="textSecondary">
              No services.
            </Typography>
          ) : (
            <List dense disablePadding aria-label="Selected IMS services">
              {selected.map((name, index) => (
                <ListItem
                  key={name}
                  divider
                  secondaryAction={
                    <>
                      <Tooltip title="Move up" arrow>
                        <span>
                          <IconButton
                            size="small"
                            aria-label={`Move ${name} up`}
                            disabled={index === 0}
                            onClick={() => move(index, -1)}
                          >
                            <ArrowUpwardIcon fontSize="small" />
                          </IconButton>
                        </span>
                      </Tooltip>
                      <Tooltip title="Move down" arrow>
                        <span>
                          <IconButton
                            size="small"
                            aria-label={`Move ${name} down`}
                            disabled={index === selected.length - 1}
                            onClick={() => move(index, 1)}
                          >
                            <ArrowDownwardIcon fontSize="small" />
                          </IconButton>
                        </span>
                      </Tooltip>
                      <Tooltip title="Remove" arrow>
                        <IconButton
                          size="small"
                          aria-label={`Remove ${name}`}
                          onClick={() =>
                            setSelected((list) =>
                              list.filter((n) => n !== name),
                            )
                          }
                        >
                          <DeleteIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    </>
                  }
                >
                  <ListItemText primary={`${index + 1}. ${name}`} />
                </ListItem>
              ))}
            </List>
          )}
          <TextField
            select
            fullWidth
            margin="normal"
            label="Add Service"
            value=""
            disabled={available.length === 0}
            helperText={
              servicesQuery.isSuccess && available.length === 0
                ? "No more services to add."
                : undefined
            }
            onChange={(e) => setSelected((list) => [...list, e.target.value])}
          >
            {available.map((s) => (
              <MenuItem key={s.name} value={s.name}>
                {s.name}
              </MenuItem>
            ))}
          </TextField>
        </Box>
      }
    />
  );
};

export default EditProfileIMSServicesModal;
