// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import CreateSubscriberModal from "./CreateSubscriberModal";
import EditSubscriberProfileModal from "./EditSubscriberProfileModal";
import EditSubscriberDescriptionModal from "./EditSubscriberDescriptionModal";
import { MAX_DESCRIPTION_LENGTH } from "@/queries/subscribers";

const api = setupApiServer();

const SUBSCRIBERS = "/api/v1/subscribers";
const PROFILES = "/api/v1/profiles";
const OPERATOR = "/api/v1/operator";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });
const field = (label: RegExp) => screen.getByLabelText(label);
const textbox = (name: RegExp) => screen.getByRole("textbox", { name });

const seed = ({ mcc = "001", mnc = "01" } = {}) => {
  api.get(OPERATOR, () => ({ id: { mcc, mnc } }));
  api.get(PROFILES, () => ({
    items: [{ name: "default" }, { name: "premium" }],
    page: 1,
    per_page: 100,
    total_count: 2,
  }));
};

const renderCreate = () => {
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  renderWithProviders(
    <CreateSubscriberModal open onClose={onClose} onSuccess={onSuccess} />,
    { auth: {} },
  );
  return { onClose, onSuccess };
};

describe("CreateSubscriberModal", () => {
  it("shows the operator prefix and preselects the first profile", async () => {
    seed();
    renderCreate();

    await screen.findByText("00101");
    await waitFor(() =>
      expect(screen.getByText("default")).toBeInTheDocument(),
    );
    expect(field(/Sequence Number/)).toHaveValue("000000000022");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("reports a load failure without blocking the dialog", async () => {
    api.get(OPERATOR, () => httpError(500, "operator unavailable"));
    api.get(PROFILES, () => httpError(500, "profiles unavailable"));
    renderCreate();

    await screen.findByText(
      "Failed to load operator or profile data. Please try again.",
    );
  });

  it("strips a pasted full IMSI down to the MSIN", async () => {
    const user = userEvent.setup();
    seed();
    renderCreate();
    await screen.findByText("00101");

    await user.type(field(/IMSI/), "001010123456789");

    await waitFor(() => expect(field(/IMSI/)).toHaveValue("0123456789"));
  });

  it("rejects an IMSI whose prefix does not match the operator", async () => {
    const user = userEvent.setup();
    seed();
    renderCreate();
    await screen.findByText("00101");

    await user.type(field(/IMSI/), "999990123456789");

    await screen.findByText("IMSI prefix does not match MCC 001 / MNC 01.");
  });

  it("Generate produces an MSIN and a key that satisfy validation", async () => {
    const user = userEvent.setup();
    seed();
    renderCreate();
    await screen.findByText("00101");

    const generates = within(dialog()).getAllByRole("button", {
      name: "Generate",
    });
    await user.click(generates[0]);
    await user.click(generates[1]);

    await waitFor(() =>
      expect((field(/^Key$/) as HTMLInputElement).value).toMatch(
        /^[0-9a-f]{32}$/,
      ),
    );
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
  });

  it("hides the OPC field until custom OPC is requested", async () => {
    const user = userEvent.setup();
    seed();
    renderCreate();
    await screen.findByText("00101");

    expect(screen.queryByLabelText(/^OPC/)).not.toBeInTheDocument();
    await user.click(screen.getByRole("checkbox", { name: /custom OPC/i }));
    expect(await screen.findByLabelText(/^OPC/)).toBeInTheDocument();

    await user.type(screen.getByLabelText(/^OPC/), "xyz");
    await user.tab();
    await screen.findByText("OPC must be empty or a 32-character hex string.");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("posts the assembled IMSI", async () => {
    const user = userEvent.setup();
    seed();
    api.post(SUBSCRIBERS, () => ({}));
    const { onClose } = renderCreate();
    await screen.findByText("00101");

    await user.type(field(/IMSI/), "0123456789");
    const generates = within(dialog()).getAllByRole("button", {
      name: "Generate",
    });
    await user.click(generates[1]);

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    const body = api.lastRequest(SUBSCRIBERS)?.body as Record<string, unknown>;
    expect(body.imsi).toBe("001010123456789");
    expect(body.profile_name).toBe("default");
    expect(body.sequenceNumber).toBe("000000000022");
  });

  it("posts the description the operator typed", async () => {
    const user = userEvent.setup();
    seed();
    api.post(SUBSCRIBERS, () => ({}));
    renderCreate();
    await screen.findByText("00101");

    await user.type(field(/IMSI/), "0123456789");
    const generates = within(dialog()).getAllByRole("button", {
      name: "Generate",
    });
    await user.click(generates[1]);
    await user.type(textbox(/Description/), "Warehouse gate reader");

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => {
      const body = api.lastRequest(SUBSCRIBERS)?.body as Record<
        string,
        unknown
      >;
      expect(body.description).toBe("Warehouse gate reader");
    });
  });

  it("rejects a description longer than the API accepts", async () => {
    const user = userEvent.setup();
    seed();
    renderCreate();
    await screen.findByText("00101");

    await user.click(textbox(/Description/));
    await user.paste("x".repeat(MAX_DESCRIPTION_LENGTH + 1));
    await user.tab();

    await screen.findByText(
      `Description must be at most ${MAX_DESCRIPTION_LENGTH} characters.`,
    );
  });
});

const IMSI = "001010123456789";
const PUT_PATH = `${SUBSCRIBERS}/${IMSI}`;

const initialData = (description = "") => ({
  imsi: IMSI,
  profileName: "default",
  description,
});

describe("EditSubscriberProfileModal", () => {
  const renderProfile = (description = "") => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditSubscriberProfileModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={initialData(description)}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("edits the profile alone", async () => {
    seed();
    renderProfile();

    expect(
      await screen.findByRole("combobox", { name: /Profile/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("textbox", { name: /Description/ }),
    ).not.toBeInTheDocument();
  });

  it("submits the chosen profile", async () => {
    const user = userEvent.setup();
    seed();
    api.put(`${SUBSCRIBERS}/:imsi`, () => ({}));
    const { onClose } = renderProfile();

    await user.click(await screen.findByRole("combobox", { name: /Profile/ }));
    await user.click(await screen.findByRole("option", { name: "premium" }));
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(PUT_PATH)?.body).toEqual({
      profile_name: "premium",
      description: "",
    });
  });

  it("carries the description through unchanged", async () => {
    const user = userEvent.setup();
    seed();
    api.put(`${SUBSCRIBERS}/:imsi`, () => ({}));
    const { onClose } = renderProfile("Warehouse gate reader");

    await user.click(await screen.findByRole("combobox", { name: /Profile/ }));
    await user.click(await screen.findByRole("option", { name: "premium" }));
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(PUT_PATH)?.body).toEqual({
      profile_name: "premium",
      description: "Warehouse gate reader",
    });
  });
});

describe("EditSubscriberDescriptionModal", () => {
  const renderDescription = (description = "") => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditSubscriberDescriptionModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={initialData(description)}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("edits the description alone", async () => {
    seed();
    renderDescription("Warehouse gate reader");

    expect(textbox(/Description/)).toHaveValue("Warehouse gate reader");
    expect(
      screen.queryByRole("combobox", { name: /Profile/ }),
    ).not.toBeInTheDocument();
  });

  it("submits the new description and carries the profile through unchanged", async () => {
    const user = userEvent.setup();
    seed();
    api.put(`${SUBSCRIBERS}/:imsi`, () => ({}));
    const { onClose } = renderDescription("Warehouse gate reader");

    await user.clear(textbox(/Description/));
    await user.type(textbox(/Description/), "Loading dock reader");
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(PUT_PATH)?.body).toEqual({
      profile_name: "default",
      description: "Loading dock reader",
    });
  });

  it("removes the description when the operator empties the field", async () => {
    const user = userEvent.setup();
    seed();
    api.put(`${SUBSCRIBERS}/:imsi`, () => ({}));
    const { onClose } = renderDescription("Warehouse gate reader");

    await user.clear(textbox(/Description/));
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(PUT_PATH)?.body).toEqual({
      profile_name: "default",
      description: "",
    });
  });

  it("rejects a description longer than the API accepts", async () => {
    const user = userEvent.setup();
    seed();
    renderDescription();

    await user.click(textbox(/Description/));
    await user.paste("x".repeat(MAX_DESCRIPTION_LENGTH + 1));
    await user.tab();

    await screen.findByText(
      `Description must be at most ${MAX_DESCRIPTION_LENGTH} characters.`,
    );
    expect(button(/^Update$/)).toBeDisabled();
  });

  it("counts characters the way the API does", async () => {
    const user = userEvent.setup();
    seed();
    renderDescription();

    // 40 emoji are 40 runes to the API and 80 UTF-16 code units to JavaScript.
    await user.click(textbox(/Description/));
    await user.paste("🙂".repeat(40));
    await user.tab();

    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
    expect(
      screen.queryByText(
        `Description must be at most ${MAX_DESCRIPTION_LENGTH} characters.`,
      ),
    ).not.toBeInTheDocument();
  });

  it("ignores surrounding whitespace the API would trim", async () => {
    const user = userEvent.setup();
    seed();
    renderDescription();

    await user.click(textbox(/Description/));
    await user.paste(`  ${"x".repeat(MAX_DESCRIPTION_LENGTH)}  `);
    await user.tab();

    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
  });
});
