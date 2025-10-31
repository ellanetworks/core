"use client";

import * as React from "react";
import { Box, IconButton, useMediaQuery } from "@mui/material";
import CloseIcon from "@mui/icons-material/Close";

type Props = {
  open: boolean;
  onClose: () => void;
  defaultWidth?: number; // px
  minWidth?: number; // px
  maxWidth?: number; // px
  children: React.ReactNode;
  header?: React.ReactNode; // optional header area
};

export default function RightSidePanel({
  open,
  onClose,
  defaultWidth = 560,
  minWidth = 360,
  maxWidth = 880,
  children,
  header,
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

  React.useEffect(() => {
    if (!open) return;
    // prevent body scroll when panel is open on small screens
    if (isSmall) {
      const prev = document.body.style.overflow;
      document.body.style.overflow = "hidden";
      return () => {
        document.body.style.overflow = prev;
      };
    }
  }, [open, isSmall]);

  const onPointerDown = (e: React.PointerEvent) => {
    // Only start when pressing on the handle (left edge)
    setDragging(true);
    startXRef.current = e.clientX;
    startWRef.current = width;
    (e.target as Element).setPointerCapture?.(e.pointerId);
  };

  const onPointerMove = (e: React.PointerEvent) => {
    if (!dragging) return;
    const dx = e.clientX - startXRef.current;
    // Handle is on the LEFT edge; moving left increases width
    const proposed = startWRef.current - dx;
    const clamped = Math.max(minWidth, Math.min(maxWidth, proposed));
    setWidth(clamped);
  };

  const onPointerUp = (e: React.PointerEvent) => {
    if (!dragging) return;
    setDragging(false);
    (e.target as Element).releasePointerCapture?.(e.pointerId);
  };

  // Mobile: use full width
  const panelWidth = isSmall ? "100%" : `${width}px`;

  return (
    <>
      {/* Backdrop (clickaway) */}
      {open && (
        <Box
          onClick={onClose}
          sx={{
            position: "fixed",
            inset: 0,
            bgcolor: "rgba(0,0,0,0.32)",
            zIndex: (t) => t.zIndex.modal + 1,
          }}
        />
      )}

      {/* Panel */}
      <Box
        role="dialog"
        aria-modal="true"
        sx={{
          position: "fixed",
          top: 0,
          right: 0,
          height: "100vh",
          width: panelWidth,
          transform: open ? "translateX(0)" : "translateX(100%)",
          transition: "transform 220ms ease",
          bgcolor: "background.paper",
          boxShadow: 8,
          zIndex: (t) => t.zIndex.modal + 2,
          display: "flex",
          flexDirection: "column",
        }}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
      >
        {/* Drag handle (hidden on small screens) */}
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
              // Wider hit target, slim visual line:
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

        {/* Content */}
        <Box sx={{ flex: 1, overflow: "auto", p: 2 }}>{children}</Box>
      </Box>
    </>
  );
}
