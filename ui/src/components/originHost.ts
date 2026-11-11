// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const fqdnRegex =
  /^(?=.{1,255}$)[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?(?:\.[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?)+$/;

export const ORIGIN_HOST_LABEL = "Origin-Host";

export const ORIGIN_HOST_HELP =
  "The Diameter host name the peer identifies itself with. Optional.";
