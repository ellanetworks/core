// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const DEFAULT_PRODUCT_NAME = "Ella Core";

export const VENDOR = {
  name: "Ella Networks",
  company: "Ella Networks Inc.",
  websiteUrl: "https://ellanetworks.com",
} as const;

export const PRODUCT = {
  name: DEFAULT_PRODUCT_NAME,
  docsUrl: "https://docs.ellanetworks.com",
};

export const logoAlt = () => `${PRODUCT.name} Logo`;

export const isRebranded = () => PRODUCT.name !== DEFAULT_PRODUCT_NAME;

export const vendorAttribution = `Powered by ${VENDOR.name}`;
