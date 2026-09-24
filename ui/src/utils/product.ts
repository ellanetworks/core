// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const DEFAULT_PRODUCT_NAME = "Ella Core";

export const VENDOR = {
  company: "Ella Networks Inc.",
  websiteUrl: "https://ellanetworks.com",
} as const;

const DOCS_URL = "https://docs.ellanetworks.com";

export const PRODUCT = {
  name: DEFAULT_PRODUCT_NAME,
  docsUrl: DOCS_URL,
  haDocsUrl: `${DOCS_URL}/explanation/high_availability/`,
  backupRestoreDocsUrl: `${DOCS_URL}/how_to/backup_and_restore/`,
};

export const logoAlt = () => `${PRODUCT.name} Logo`;
