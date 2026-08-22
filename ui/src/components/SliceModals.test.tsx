// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import CreateSliceModal from "./CreateSliceModal";
import EditSliceModal from "./EditSliceModal";

const api = setupApiServer();

const PATH = "/api/v1/slices";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });
const field = (label: RegExp) => screen.getByLabelText(label);
const setSst = (value: string) => {
  fireEvent.change(field(/SST/), { target: { value } });
  fireEvent.blur(field(/SST/));
};

const renderCreate = () => {
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  renderWithProviders(
    <CreateSliceModal open onClose={onClose} onSuccess={onSuccess} />,
    { auth: {} },
  );
  return { onClose, onSuccess };
};

describe("CreateSliceModal", () => {
  it("prefills sst and shows both hints", () => {
    renderCreate();
    expect(field(/SST/)).toHaveValue(1);
    expect(screen.getByText("Slice/Service Type (0–255)")).toBeInTheDocument();
    expect(
      screen.getByText("Slice Differentiator — 6 hex digits (e.g. 010203)"),
    ).toBeInTheDocument();
    expect(dialog()).toHaveAccessibleName("Create Network Slice");
  });

  it("rejects an sst outside 0-255", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.type(field(/Name/), "slice-a");
    setSst("300");

    await screen.findByText("SST must be between 0 and 255");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("rejects an sd that is not 6 hex digits", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.type(field(/Name/), "slice-a");
    await user.type(field(/SD/), "zzz");
    await user.tab();

    await screen.findByText("SD must be a 6-digit hex string");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("omits an empty sd from the request", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({}));
    const { onClose, onSuccess } = renderCreate();

    await user.type(field(/Name/), "slice-a");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSuccess).toHaveBeenCalled();
    expect(api.lastRequest(PATH)?.body).toEqual({ name: "slice-a", sst: 1 });
  });

  it("includes a valid sd and sends sst as a number", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({}));
    renderCreate();

    await user.type(field(/Name/), "slice-b");
    setSst("128");
    await user.type(field(/SD/), "010203");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() =>
      expect(api.lastRequest(PATH)?.body).toEqual({
        name: "slice-b",
        sst: 128,
        sd: "010203",
      }),
    );
  });

  it("keeps the dialog open when the API rejects the create", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => httpError(409, "slice already exists"));
    const { onClose } = renderCreate();

    await user.type(field(/Name/), "slice-a");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await screen.findByText(
      /Failed to create network slice: .*slice already exists/,
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("EditSliceModal", () => {
  const initialData = { name: "slice-a", sst: 1, sd: "010203" };

  it("seeds from initialData and locks the name", () => {
    renderWithProviders(
      <EditSliceModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        initialData={initialData}
      />,
      { auth: {} },
    );

    expect(field(/Name/)).toHaveValue("slice-a");
    expect(field(/Name/)).toBeDisabled();
    expect(field(/SD/)).toHaveValue("010203");
    expect(dialog()).toHaveAccessibleName("Edit Network Slice");
  });

  it("puts the edited sst without a name in the body", async () => {
    const user = userEvent.setup();
    api.put(`${PATH}/:name`, () => ({}));
    const onClose = vi.fn();

    renderWithProviders(
      <EditSliceModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={initialData}
      />,
      { auth: {} },
    );

    setSst("2");
    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(`${PATH}/slice-a`)?.body).toEqual({
      sst: 2,
      sd: "010203",
    });
  });
});
