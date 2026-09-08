// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { logoAlt } from "@/utils/product";

export const MARK_MAX_SIZE = 64;

export default function Logo({
  width = 50,
  height = 50,
}: {
  width?: number;
  height?: number;
}) {
  const src =
    Math.max(width, height) <= MARK_MAX_SIZE ? "/logo-mark.svg" : "/logo.svg";
  return <img src={src} alt={logoAlt()} width={width} height={height} />;
}
