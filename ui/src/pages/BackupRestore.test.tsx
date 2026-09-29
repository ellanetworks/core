// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, expect, it } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { httpError, setupApiServer } from "@/test/apiServer";
import { renderWithProviders } from "@/test/renderWithProviders";
import BackupRestore from "./BackupRestore";

const api = setupApiServer();

const restoreBackupAheadMessage =
  "This backup cannot be restored online because it is newer than the current database state. Use offline restore instead.";

describe("BackupRestore", () => {
  it("advises offline restore when the uploaded backup is ahead of the current state", async () => {
    api.get("/api/v1/status", () => ({
      initialized: true,
      ready: true,
      schemaVersion: 20,
      cluster: { enabled: false },
    }));
    api.post("/api/v1/restore", () =>
      httpError(409, restoreBackupAheadMessage),
    );

    const user = userEvent.setup();
    const { container } = renderWithProviders(<BackupRestore />, { auth: {} });

    await screen.findByRole("button", { name: "Upload File" });

    const fileInput = container.querySelector('input[type="file"]');
    if (!(fileInput instanceof HTMLInputElement)) {
      throw new Error("restore file input was not rendered");
    }

    await user.upload(
      fileInput,
      new File(["backup"], "backup.backup", { type: "application/gzip" }),
    );

    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Restore" }));

    expect(await screen.findByText(/Use offline restore instead/)).toBeTruthy();
  });
});
