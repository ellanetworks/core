// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const DEFAULT_PRODUCT_NAME = "Ella Core";

export const VENDOR = {
  company: "Ella Networks Inc.",
  websiteUrl: "https://ellanetworks.com",
} as const;

export const PRODUCT = {
  name: DEFAULT_PRODUCT_NAME,
  docsUrl: "https://docs.ellanetworks.com",
};

export const logoAlt = () => `${PRODUCT.name} Logo`;
