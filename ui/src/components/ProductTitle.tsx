// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Box, Typography } from "@mui/material";
import { PRODUCT, VENDOR, logoHeight } from "@/utils/product";

export default function ProductTitle() {
  if (!PRODUCT.showProductName && !PRODUCT.poweredBy) {
    return null;
  }

  return (
    <Box
      sx={{
        ml: 2,
        minWidth: 0,
        display: "flex",
        flexDirection: "column",
        justifyContent: PRODUCT.showProductName ? "center" : "flex-end",
        height: PRODUCT.showProductName ? undefined : logoHeight(),
      }}
    >
      {PRODUCT.showProductName && (
        <Typography variant="h6" noWrap component="div">
          {PRODUCT.name}
        </Typography>
      )}
      {PRODUCT.poweredBy && (
        <Typography
          variant="caption"
          noWrap
          component="div"
          sx={{ lineHeight: 1 }}
        >
          Powered by {VENDOR.name}
        </Typography>
      )}
    </Box>
  );
}
