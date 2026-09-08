// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Box, Typography } from "@mui/material";
import { PRODUCT, isRebranded, vendorAttribution } from "@/utils/product";

export default function ProductTitle() {
  return (
    <Box sx={{ ml: 2, minWidth: 0 }}>
      <Typography variant="h6" noWrap component="div" sx={{ lineHeight: 1.2 }}>
        {PRODUCT.name}
      </Typography>
      {isRebranded() && (
        <Typography
          variant="caption"
          noWrap
          component="div"
          sx={{ opacity: 0.8 }}
        >
          {vendorAttribution}
        </Typography>
      )}
    </Box>
  );
}
