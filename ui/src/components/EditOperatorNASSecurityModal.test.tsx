// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import EditOperatorNASSecurityModal from "./EditOperatorNASSecurityModal";

const api = setupApiServer();

const PATH = "/api/v1/operator/nas-security";

const dialog = () => screen.getByRole("dialog");
const updateButton = () =>
  within(dialog()).getByRole("button", { name: /^Update$/ });

const section = (title: string) =>
  screen.getByText(title).closest("div") as HTMLElement;

const checkbox = (title: string, algorithm: string) =>
  within(section(title)).getByRole("checkbox", { name: algorithm });

const render = (
  ciphering = ["AES", "SNOW3G"],
  integrity = ["AES", "SNOW3G"],
) => {
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  renderWithProviders(
    <EditOperatorNASSecurityModal
      open
      onClose={onClose}
      onSuccess={onSuccess}
      initialData={{ ciphering, integrity }}
    />,
    { auth: {} },
  );
  return { onClose, onSuccess };
};

describe("EditOperatorNASSecurityModal", () => {
  it("checks enabled algorithms and leaves the rest unchecked", () => {
    render(["AES"], ["SNOW3G"]);

    expect(checkbox("Ciphering Preference", "AES")).toBeChecked();
    expect(checkbox("Ciphering Preference", "NULL")).not.toBeChecked();
    expect(checkbox("Integrity Preference", "SNOW3G")).toBeChecked();
    expect(dialog()).toHaveAccessibleName("Edit NAS Security Algorithms");
  });

  it("blocks submit when a list has nothing enabled", async () => {
    const user = userEvent.setup();
    render(["AES"], ["AES"]);

    await user.click(checkbox("Ciphering Preference", "AES"));

    await screen.findByText("At least one algorithm must be enabled.");
    await waitFor(() => expect(updateButton()).toBeDisabled());
  });

  it("submits only enabled algorithms, in list order", async () => {
    const user = userEvent.setup();
    api.put(PATH, () => ({}));
    const { onClose, onSuccess } = render(["AES", "SNOW3G"], ["AES"]);

    await user.click(checkbox("Integrity Preference", "SNOW3G"));
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSuccess).toHaveBeenCalled();
    expect(api.lastRequest(PATH)?.body).toEqual({
      ciphering: ["AES", "SNOW3G"],
      integrity: ["AES", "SNOW3G"],
    });
  });

  it("warns when NULL is the preferred algorithm", () => {
    render(["NULL", "AES"], ["AES"]);
    expect(
      screen.getByText(/NULL is the preferred ciphering algorithm/),
    ).toBeInTheDocument();
  });

  it("warns differently when NULL is only a fallback", () => {
    render(["AES", "NULL"], ["AES"]);
    expect(
      screen.getByText(/NULL is enabled as a fallback/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/NULL is the preferred/)).not.toBeInTheDocument();
  });

  it("shows no NULL warning when NULL is disabled everywhere", () => {
    render(["AES"], ["AES"]);
    expect(screen.queryByText(/NULL is/)).not.toBeInTheDocument();
  });

  it("names both algorithm kinds when NULL leads both lists", () => {
    render(["NULL"], ["NULL"]);
    expect(
      screen.getByText(/NULL is the preferred ciphering and integrity/),
    ).toBeInTheDocument();
  });

  it("keeps the dialog open and reports a failed update", async () => {
    const user = userEvent.setup();
    api.put(PATH, () => httpError(500, "algorithms rejected"));
    const { onClose } = render();

    await user.click(updateButton());

    await screen.findByText(
      /Failed to update security algorithms: .*algorithms rejected/,
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});
