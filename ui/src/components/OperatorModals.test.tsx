// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import EditOperatorIdModal from "./EditOperatorIdModal";
import EditOperatorCodeModal from "./EditOperatorCodeModal";
import EditOperatorSPNModal from "./EditOperatorSPNModal";
import EditOperatorTrackingModal from "./EditOperatorTrackingModal";

const api = setupApiServer();

const dialog = () => screen.getByRole("dialog");
const updateButton = () =>
  within(dialog()).getByRole("button", { name: /^Update$/ });
const field = (label: RegExp) => screen.getByLabelText(label);

const retype = async (
  user: ReturnType<typeof userEvent.setup>,
  label: RegExp,
  value: string,
) => {
  await user.clear(field(label));
  if (value) await user.type(field(label), value);
  await user.tab();
};

describe("EditOperatorIdModal", () => {
  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorIdModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={{ mcc: "001", mnc: "01" }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("seeds both fields and resolves its description", () => {
    render();
    expect(field(/MCC/)).toHaveValue("001");
    expect(field(/MNC/)).toHaveValue("01");
    expect(dialog()).toHaveAccessibleName("Edit Operator ID");

    const describedBy = dialog().getAttribute("aria-describedby");
    expect(document.getElementById(describedBy!)).toHaveTextContent(
      /Mobile Country Code \(MCC\)/,
    );
  });

  it("rejects an MCC that is not three digits", async () => {
    const user = userEvent.setup();
    render();

    await retype(user, /MCC/, "12");
    await screen.findByText("MCC must be a 3 decimal digit");
    expect(updateButton()).toBeDisabled();
  });

  it("accepts a three digit MNC", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/operator/id", () => ({}));
    const { onClose } = render();

    await retype(user, /MNC/, "010");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest("/api/v1/operator/id")?.body).toEqual({
      mcc: "001",
      mnc: "010",
    });
  });
});

describe("EditOperatorCodeModal", () => {
  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorCodeModal open onClose={onClose} onSuccess={vi.fn()} />,
      { auth: {} },
    );
    return { onClose };
  };

  it("warns that the operation cannot be undone", () => {
    render();
    expect(
      screen.getByText(/This operation cannot be undone/),
    ).toBeInTheDocument();
    expect(updateButton()).toBeDisabled();
  });

  it("requires a 32 character hex code", async () => {
    const user = userEvent.setup();
    render();

    await retype(user, /^Operator Code$/, "nothex");
    await screen.findByText(
      "Operator Code must be a 32-character hexadecimal string.",
    );
    expect(updateButton()).toBeDisabled();

    await retype(user, /^Operator Code$/, "0".repeat(32));
    await waitFor(() => expect(updateButton()).toBeEnabled());
  });

  it("submits the code", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/operator/code", () => ({}));
    const { onClose } = render();

    const code = "0123456789abcdef0123456789ABCDEF";
    await retype(user, /^Operator Code$/, code);
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest("/api/v1/operator/code")?.body).toEqual({
      operatorCode: code,
    });
  });
});

describe("EditOperatorSPNModal", () => {
  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorSPNModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={{ fullName: "Ella", shortName: "Ella" }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("shows both length hints until a field is invalid", async () => {
    const user = userEvent.setup();
    render();

    expect(
      screen.getByText(/The full network name shown on UE displays/),
    ).toBeInTheDocument();

    await retype(user, /Full Name/, "");
    await screen.findByText("Full name is required");
    expect(updateButton()).toBeDisabled();
  });

  it("caps input at the maximum length", () => {
    render();
    expect(field(/Full Name/)).toHaveAttribute("maxlength", "50");
    expect(field(/Short Name/)).toHaveAttribute("maxlength", "50");
  });

  it("submits both names with the operator SPN error prefix on failure", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/operator/spn", () => httpError(500, "spn rejected"));
    const { onClose } = render();

    await retype(user, /Short Name/, "EN");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await screen.findByText(/Failed to update operator SPN: .*spn rejected/);
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("EditOperatorTrackingModal", () => {
  const TRACKING_PATH = "/api/v1/operator/tracking";

  const render = (supportedTacs = ["000001"]) => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorTrackingModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={{ supportedTacs }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("renders existing TACs as chips with the default hint", () => {
    render(["000001", "0000FF"]);
    expect(
      screen.getByText(/Enter each TAC as a 3 bytes hex string/),
    ).toBeInTheDocument();
    expect(dialog()).toHaveAccessibleName("Edit Operator Tracking Information");
  });

  it("shows a decimal hint while typing a valid TAC", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Supported TACs/), "000010");
    await screen.findByText("000010 is 16 in decimal");
  });

  it("commits a pending TAC that was never turned into a chip", async () => {
    const user = userEvent.setup();
    api.put(TRACKING_PATH, () => ({}));
    const { onClose } = render(["000001"]);

    await user.type(field(/Supported TACs/), "000002");
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(TRACKING_PATH)?.body).toEqual({
      supportedTacs: ["000001", "000002"],
    });
  });

  it("rejects a pending TAC that is not valid hex and stays open", async () => {
    const user = userEvent.setup();
    const { onClose } = render(["000001"]);

    await user.type(field(/Supported TACs/), "zzzzzz");
    await user.click(updateButton());

    await screen.findByText("Invalid TACs: zzzzzz");
    expect(onClose).not.toHaveBeenCalled();
    expect(api.requests(TRACKING_PATH)).toHaveLength(0);
  });

  it("submits chips added with Enter", async () => {
    const user = userEvent.setup();
    api.put(TRACKING_PATH, () => ({}));
    const { onClose } = render([]);

    await user.type(field(/Supported TACs/), "00000A{Enter}");
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(TRACKING_PATH)?.body).toEqual({
      supportedTacs: ["00000A"],
    });
  });
});
