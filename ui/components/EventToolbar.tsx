"use client";

import * as React from "react";
import { Box, Button, Tooltip, Typography } from "@mui/material";
import { Toolbar } from "@mui/x-data-grid";
import EditIcon from "@mui/icons-material/Edit";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutline";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import PauseIcon from "@mui/icons-material/Pause";
import FiberManualRecordIcon from "@mui/icons-material/FiberManualRecord";

export type EventToolbarState = {
  canEdit: boolean;
  retentionDays?: number | string;
  onEditRetention: () => void;
  onClearAll: () => void;
  isLive: boolean;
  onToggleLive: () => void;
};

export const EventToolbarContext =
  React.createContext<EventToolbarState | null>(null);

export function useEventToolbar() {
  const ctx = React.useContext(EventToolbarContext);
  if (!ctx) throw new Error("EventToolbarContext missing");
  return ctx;
}

export function EventToolbar() {
  const {
    canEdit,
    retentionDays,
    onEditRetention,
    onClearAll,
    isLive,
    onToggleLive,
  } = useEventToolbar();

  return (
    <Toolbar>
      <Box
        sx={{
          position: "sticky",
          top: 0,
          zIndex: (t) => t.zIndex.appBar,
          bgcolor: "background.paper",
          display: "flex",
          alignItems: "center",
          gap: 1.25,
          px: 1.25,
          py: 1,
          whiteSpace: "nowrap",
          overflow: "hidden",
          width: "100%",
        }}
      >
        {canEdit && (
          <Button
            variant="outlined"
            color="error"
            size="small"
            startIcon={<DeleteOutlineIcon />}
            onClick={onClearAll}
            sx={{ flexShrink: 0 }}
          >
            Clear All
          </Button>
        )}

        <Box sx={{ flex: 1, minWidth: 8 }} />

        <Typography
          variant="body2"
          color="text.secondary"
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 0.75,
            overflow: "hidden",
            textOverflow: "ellipsis",
          }}
          title={`Retention: ${retentionDays ?? "…"} days`}
        >
          Retention: <strong>{retentionDays ?? "…"}</strong> days
          {canEdit && (
            <Button
              variant="text"
              size="small"
              startIcon={<EditIcon fontSize="small" />}
              onClick={onEditRetention}
              sx={{ minWidth: 0, px: 0.75, flexShrink: 0 }}
            >
              Edit
            </Button>
          )}
        </Typography>

        <Box
          sx={{
            width: 1,
            maxWidth: 12,
            height: 24,
            borderLeft: 1,
            borderColor: "divider",
            mx: 1,
            flexShrink: 0,
          }}
        />

        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 0.5,
            flexShrink: 0,
          }}
        >
          <FiberManualRecordIcon
            fontSize="small"
            sx={{ color: isLive ? "success.main" : "error.main", opacity: 0.9 }}
            aria-label={isLive ? "Live" : "Paused"}
          />
          <Tooltip title={isLive ? "Pause updates" : "Resume updates"}>
            <span>
              <Button
                size="small"
                variant="text"
                onClick={onToggleLive}
                startIcon={
                  isLive ? (
                    <PauseIcon fontSize="small" />
                  ) : (
                    <PlayArrowIcon fontSize="small" />
                  )
                }
                sx={{ minWidth: 0 }}
                aria-label={
                  isLive ? "Pause live updates" : "Resume live updates"
                }
              />
            </span>
          </Tooltip>
        </Box>
      </Box>
    </Toolbar>
  );
}
