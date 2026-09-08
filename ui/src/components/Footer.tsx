// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Box, Container, Typography, Link } from "@mui/material";
import { MAX_WIDTH } from "@/utils/layout";
import { PRODUCT } from "@/utils/product";

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
      <Container maxWidth={false} sx={{ maxWidth: MAX_WIDTH }}>
        <Typography
          variant="body2"
          color="textSecondary"
          sx={{
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: "6px",
          }}
        >
          © 2026 {PRODUCT.company}
          <span>·</span>
          <Link
            href={PRODUCT.websiteUrl}
            target="_blank"
            rel="noopener noreferrer"
            color="textSecondary"
            underline="hover"
          >
            {PRODUCT.websiteUrl.replace("https://", "")}
          </Link>
        </Typography>
      </Container>
    </Box>
  );
}
