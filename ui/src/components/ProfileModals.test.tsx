// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import CreateProfileModal from "./CreateProfileModal";
import EditProfileModal from "./EditProfileModal";
import type { APIProfile } from "@/queries/profiles";

const api = setupApiServer();

const PROFILES = "/api/v1/profiles";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });
const field = (label: RegExp) => screen.getByLabelText(label);

describe("CreateProfileModal", () => {
  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <CreateProfileModal open onClose={onClose} onSuccess={vi.fn()} />,
      { auth: {} },
    );
    return { onClose };
  };

  it("defaults to 100 Mbps both ways with both access types allowed", () => {
    render();
    expect(field(/Bitrate Uplink/)).toHaveValue(100);
    expect(field(/Bitrate Downlink/)).toHaveValue(100);
    expect(
      screen.getByRole("checkbox", { name: "Allow 4G access" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Allow 5G access" }),
    ).toBeChecked();
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("rejects a bitrate outside 1-65535", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Name/), "gold");
    fireEvent.change(field(/Bitrate Uplink/), { target: { value: "70000" } });
    fireEvent.blur(field(/Bitrate Uplink/));

    await screen.findByText("Value must be between 1 and 65535");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("rejects a fractional bitrate", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Name/), "gold");
    fireEvent.change(field(/Bitrate Downlink/), { target: { value: "1.5" } });
    fireEvent.blur(field(/Bitrate Downlink/));

    await screen.findByText("Value must be a whole number");
  });

  it("joins value and unit into the AMBR strings", async () => {
    const user = userEvent.setup();
    api.post(PROFILES, () => ({}));
    const { onClose } = render();

    await user.type(field(/Name/), "gold");
    fireEvent.change(field(/Bitrate Uplink/), { target: { value: "50" } });

    const unitSelects = within(dialog()).getAllByRole("combobox", {
      name: /Unit/,
    });
    await user.click(unitSelects[0]);
    await user.click(await screen.findByRole("option", { name: "Gbps" }));

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(PROFILES)?.body).toMatchObject({
      name: "gold",
      ue_ambr_uplink: "50 Gbps",
      ue_ambr_downlink: "100 Mbps",
    });
  });

  it("sends the access flags as booleans", async () => {
    const user = userEvent.setup();
    api.post(PROFILES, () => ({}));
    render();

    await user.type(field(/Name/), "lte-only");
    await user.click(screen.getByRole("checkbox", { name: "Allow 5G access" }));

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => {
      const body = api.lastRequest(PROFILES)?.body as Record<string, unknown>;
      expect(body.allow_4g).toBe(true);
      expect(body.allow_5g).toBe(false);
    });
  });

  it("keeps the dialog open on a failed create", async () => {
    const user = userEvent.setup();
    api.post(PROFILES, () => httpError(409, "profile already exists"));
    const { onClose } = render();

    await user.type(field(/Name/), "gold");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await screen.findByText(
      /Failed to create profile: .*profile already exists/,
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("EditProfileModal", () => {
  const profile = {
    name: "gold",
    ue_ambr_uplink: "2 Gbps",
    ue_ambr_downlink: "500 Mbps",
    allow_4g: false,
    allow_5g: true,
  } as APIProfile;

  it("parses the stored AMBR strings back into value and unit", () => {
    renderWithProviders(
      <EditProfileModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        initialData={profile}
      />,
      { auth: {} },
    );

    expect(field(/Name/)).toHaveValue("gold");
    expect(field(/Name/)).toBeDisabled();
    expect(field(/Bitrate Uplink/)).toHaveValue(2);
    expect(field(/Bitrate Downlink/)).toHaveValue(500);
    expect(
      screen.getByRole("checkbox", { name: "Allow 4G access" }),
    ).not.toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Allow 5G access" }),
    ).toBeChecked();
  });

  it("falls back to 100 Mbps for an unparseable AMBR string", () => {
    renderWithProviders(
      <EditProfileModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        initialData={{ ...profile, ue_ambr_uplink: "garbage" }}
      />,
      { auth: {} },
    );

    expect(field(/Bitrate Uplink/)).toHaveValue(100);
  });

  it("puts the edited profile under its original name", async () => {
    const user = userEvent.setup();
    api.put(`${PROFILES}/:name`, () => ({}));
    const onClose = vi.fn();

    renderWithProviders(
      <EditProfileModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={profile}
      />,
      { auth: {} },
    );

    fireEvent.change(field(/Bitrate Downlink/), { target: { value: "750" } });
    await waitFor(() => expect(button(/^Save$/)).toBeEnabled());
    await user.click(button(/^Save$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(`${PROFILES}/gold`)?.body).toMatchObject({
      ue_ambr_uplink: "2 Gbps",
      ue_ambr_downlink: "750 Mbps",
    });
  });
});
