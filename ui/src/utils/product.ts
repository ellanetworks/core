// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const DEFAULT_PRODUCT_NAME = BRANDING.appName;

export const VENDOR = {
  name: "Ella Networks",
  company: "Ella Networks Inc.",
  websiteUrl: "https://ellanetworks.com",
} as const;

const DOCS_URL = "https://docs.ellanetworks.com";

export const PRODUCT = {
  name: DEFAULT_PRODUCT_NAME,
  showProductName: BRANDING.showProductName,
  poweredBy: BRANDING.poweredBy,
  docsUrl: DOCS_URL,
  haDocsUrl: `${DOCS_URL}/explanation/high_availability/`,
  backupRestoreDocsUrl: `${DOCS_URL}/how_to/backup_and_restore/`,
};

export const logoHeight = () => (PRODUCT.showProductName ? 50 : 32);

export const logoAlt = () =>
  PRODUCT.showProductName ? `${PRODUCT.name} Logo` : PRODUCT.name;
