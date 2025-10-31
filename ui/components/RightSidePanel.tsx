"use client";

import * as React from "react";
import {
  Box,
  IconButton,
  ClickAwayListener,
  useMediaQuery,
} from "@mui/material";
import CloseIcon from "@mui/icons-material/Close";

type Props = {
  open: boolean;
  onClose: () => void;
  defaultWidth?: number;
  minWidth?: number;
  maxWidth?: number;
  header?: React.ReactNode;
  children?: React.ReactNode;
};

export default function RightSidePanelInline({
  open,
  onClose,
  defaultWidth = 560,
  minWidth = 360,
  maxWidth = 880,
  header,
  children,
}: Props) {
  const isSmall = useMediaQuery("(max-width:900px)");
  const [width, setWidth] = React.useState(defaultWidth);
  const [dragging, setDragging] = React.useState(false);
  const startXRef = React.useRef(0);
  const startWRef = React.useRef(width);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    if (open) window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  const onPointerDown = (e: React.PointerEvent) => {
    setDragging(true);
    startXRef.current = e.clientX;
    startWRef.current = width;
    (e.target as Element).setPointerCapture?.(e.pointerId);
  };
  const onPointerMove = (e: React.PointerEvent) => {
    if (!dragging) return;
    const dx = e.clientX - startXRef.current; // dragging handle on LEFT edge
    const proposed = startWRef.current - dx;
    const clamped = Math.max(minWidth, Math.min(maxWidth, proposed));
    setWidth(clamped);
  };
  const onPointerUp = (e: React.PointerEvent) => {
    if (!dragging) return;
    setDragging(false);
    (e.target as Element).releasePointerCapture?.(e.pointerId);
  };

  // Mobile: panel becomes full-width row below/above (no resize)
  const panelWidth = isSmall ? "100%" : `${width}px`;

  if (!open) return null; // don’t take space when closed

  return (
    <ClickAwayListener
      onClickAway={(e) => {
        // ignore click-away when dragging the handle
        if (!dragging) onClose();
      }}
    >
      <Box
        role="dialog"
        aria-modal="false"
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        sx={{
          width: panelWidth,
          minWidth: isSmall ? "100%" : `${minWidth}px`,
          maxWidth: isSmall ? "100%" : `${maxWidth}px`,
          height: "100%",
          display: "flex",
          flexDirection: "column",
          bgcolor: "background.paper",
          borderLeft: (t) => `1px solid ${t.palette.divider}`,
          boxShadow: isSmall ? 0 : 2,
          transition: "width 200ms ease",
          position: "relative",
        }}
      >
        {/* Resize handle (desktop only) */}
        {!isSmall && (
          <Box
            aria-label="Resize"
            title="Drag to resize"
            onPointerDown={onPointerDown}
            sx={{
              position: "absolute",
              left: -6,
              top: 0,
              width: 12,
              height: "100%",
              cursor: "ew-resize",
              "&::after": {
                content: '""',
                position: "absolute",
                left: 5,
                top: 0,
                width: 1,
                height: "100%",
                bgcolor: "divider",
              },
            }}
          />
        )}

        {/* Header */}
        <Box
          sx={{
            position: "sticky",
            top: 0,
            zIndex: 1,
            px: 2,
            py: 1.5,
            borderBottom: (t) => `1px solid ${t.palette.divider}`,
            display: "flex",
            alignItems: "center",
            gap: 1,
          }}
        >
          <Box sx={{ flex: 1, minWidth: 0 }}>{header}</Box>
          <IconButton aria-label="Close panel" onClick={onClose}>
            <CloseIcon />
          </IconButton>
        </Box>

        {/* Body */}
        <Box sx={{ flex: 1, overflow: "auto", p: 2 }}>{children}</Box>
      </Box>
    </ClickAwayListener>
  );
}
