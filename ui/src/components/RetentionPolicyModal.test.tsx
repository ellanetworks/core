// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import EditAuditLogRetentionPolicyModal from "./EditAuditLogRetentionPolicyModal";
import EditRadioEventRetentionPolicyModal from "./EditRadioEventRetentionPolicyModal";
import EditUsageRetentionPolicyModal from "./EditUsageRetentionPolicyModal";
import EditFlowReportsRetentionPolicyModal from "./EditFlowReportsRetentionPolicyModal";

const api = setupApiServer();

const AUDIT_PATH = "/api/v1/logs/audit/retention";

const dialog = () => screen.getByRole("dialog");
const updateButton = () =>
  within(dialog()).getByRole("button", { name: /^Update$/ });
const daysField = () => screen.getByLabelText(/Days/);

const renderAudit = (days = 30) => {
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  renderWithProviders(
    <EditAuditLogRetentionPolicyModal
      open
      onClose={onClose}
      onSuccess={onSuccess}
      initialData={{ days }}
    />,
    { auth: {} },
  );
  return { onClose, onSuccess };
};

const setDays = (value: string) =>
  fireEvent.change(daysField(), { target: { value } });

describe("RetentionPolicyModal", () => {
  it("seeds the field from the current policy", () => {
    renderAudit(45);
    expect(daysField()).toHaveValue(45);
    expect(dialog()).toHaveAccessibleName("Edit Audit Log Retention Policy");
  });

  it("resolves its aria-describedby to real description text", () => {
    renderAudit();
    const describedBy = dialog().getAttribute("aria-describedby");
    expect(describedBy).toBeTruthy();
    const target = document.getElementById(describedBy!);
    expect(target).toBeInTheDocument();
    expect(target).toHaveTextContent(/Set the number of days to retain/);
  });

  it("rejects values outside the allowed range", async () => {
    renderAudit();

    setDays("0");
    await screen.findByText("Minimum retention is 1 day");
    expect(updateButton()).toBeDisabled();

    setDays("4000");
    await screen.findByText("Maximum retention is 3650 days (10 years)");
    expect(updateButton()).toBeDisabled();

    setDays("60");
    await waitFor(() => expect(updateButton()).toBeEnabled());
  });

  it("rejects a non-integer value", async () => {
    renderAudit();

    setDays("1.5");
    await screen.findByText("Days must be a whole number");
    expect(updateButton()).toBeDisabled();
  });

  it("warns before a reduction and names what will be deleted", async () => {
    renderAudit(30);

    expect(screen.queryByText(/Reducing retention/)).not.toBeInTheDocument();

    setDays("7");
    expect(await screen.findByText(/Reducing retention/)).toHaveTextContent(
      "Reducing retention from 30 to 7 days will permanently delete logs older than 7 days.",
    );
  });

  it("does not warn when retention is increased", async () => {
    renderAudit(30);

    setDays("90");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    expect(screen.queryByText(/Reducing retention/)).not.toBeInTheDocument();
  });

  it("submits the value as a number and closes", async () => {
    const user = userEvent.setup();
    api.put(AUDIT_PATH, () => ({}));
    const { onClose, onSuccess } = renderAudit(30);

    setDays("90");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSuccess).toHaveBeenCalled();
    expect(api.lastRequest(AUDIT_PATH)?.body).toEqual({ days: 90 });
  });

  it("reports a failed update with the resource-specific prefix", async () => {
    const user = userEvent.setup();
    api.put(AUDIT_PATH, () => httpError(500, "database is locked"));
    const { onClose } = renderAudit(30);

    setDays("90");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await screen.findByText(
      /Failed to update audit log retention policy: .*database is locked/,
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("retention policy wrappers", () => {
  it("radio events keeps its initialDays prop shape and endpoint", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/ran/events/retention", () => ({}));
    const onClose = vi.fn();

    renderWithProviders(
      <EditRadioEventRetentionPolicyModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialDays={7}
      />,
      { auth: {} },
    );

    expect(dialog()).toHaveAccessibleName("Edit Network Log Retention Policy");
    setDays("3");
    expect(await screen.findByText(/Reducing retention/)).toHaveTextContent(
      "permanently delete radio events older than 3 days",
    );

    await user.click(updateButton());
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest("/api/v1/ran/events/retention")?.body).toEqual({
      days: 3,
    });
  });

  it("usage and flow reports target their own endpoints", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/subscriber-usage/retention", () => ({}));
    const onClose = vi.fn();

    const { unmount } = renderWithProviders(
      <EditUsageRetentionPolicyModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={{ days: 30 }}
      />,
      { auth: {} },
    );
    setDays("10");
    await user.click(updateButton());
    await waitFor(() =>
      expect(
        api.lastRequest("/api/v1/subscriber-usage/retention")?.body,
      ).toEqual({ days: 10 }),
    );
    unmount();

    api.put("/api/v1/flow-reports/retention", () => ({}));
    renderWithProviders(
      <EditFlowReportsRetentionPolicyModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        initialData={{ days: 30 }}
      />,
      { auth: {} },
    );
    setDays("12");
    await user.click(updateButton());
    await waitFor(() =>
      expect(api.lastRequest("/api/v1/flow-reports/retention")?.body).toEqual({
        days: 12,
      }),
    );
  });
});
