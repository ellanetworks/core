// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Typography } from "@mui/material";
import { PRODUCT } from "@/utils/product";

export default function ProductTitle() {
  return (
    <Typography variant="h6" noWrap component="div" sx={{ ml: 2 }}>
      {PRODUCT.name}
    </Typography>
  );
}
