"use client";

import { Box, Container, Typography } from "@mui/material";

export default function Footer() {
  return (
    <Box
      component="footer"
      sx={{
        mt: "auto",
        borderTop: 1,
        borderColor: "divider",
        py: 2,
        bgcolor: "background.paper",
      }}
    >
      <Container maxWidth="lg">
        <Typography variant="body2" color="text.secondary">
          © 2025 Ella Networks Inc.
        </Typography>
      </Container>
    </Box>
  );
}
