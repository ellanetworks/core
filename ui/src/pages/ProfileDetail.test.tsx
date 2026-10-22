// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import ProfileDetail from "./ProfileDetail";

const api = setupApiServer();

const IFC_PATH = "/api/v1/profiles/default/ims-filter-criteria";
const AS_PATH = "/api/v1/operator/ims/application-servers";

const registerIFC = {
  application_server_name: "voicemail",
  trigger_point: {
    condition_type_cnf: false,
    spt: [{ group: [0], method: "REGISTER" }],
  },
  default_handling: "SESSION_CONTINUED",
};

const terminatingIFC = {
  application_server_name: "conf",
  trigger_point: {
    condition_type_cnf: false,
    spt: [{ group: [0], session_case: "TERMINATING_UNREGISTERED" }],
  },
  default_handling: "SESSION_TERMINATED",
};

const mcpttPresetIFCs = [
  {
    application_server_name: "mcptt",
    trigger_point: {
      condition_type_cnf: false,
      spt: [{ group: [0], method: "REGISTER" }],
    },
    default_handling: "SESSION_CONTINUED",
    include_register_request: true,
    include_register_response: true,
  },
  {
    application_server_name: "mcptt",
    trigger_point: {
      condition_type_cnf: true,
      spt: [
        { group: [0], session_case: "ORIGINATING_REGISTERED" },
        {
          group: [1],
          sip_header: {
            header: "^P-Asserted-Service$",
            content: "^urn:urn-7:3gpp-service\\.ims\\.icsi\\.mcptt$",
          },
        },
      ],
    },
    default_handling: "SESSION_TERMINATED",
  },
];

beforeEach(() => {
  api.get("/api/v1/profiles/default", () => ({
    name: "default",
    ue_ambr_uplink: "1 Gbps",
    ue_ambr_downlink: "1 Gbps",
    allow_4g: true,
    allow_5g: true,
  }));
  api.get("/api/v1/policies", () => ({
    items: [],
    page: 1,
    per_page: 100,
    total_count: 0,
  }));
  api.get(AS_PATH, () => ({
    items: [
      {
        name: "mcptt",
        server_name: "sip:mcptt.ims.example.org",
        public_service_identities: [],
      },
      {
        name: "voicemail",
        server_name: "sip:vm.ims.example.org",
        public_service_identities: [],
      },
    ],
  }));
});

const renderProfile = (role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route path="/profiles/:name" element={<ProfileDetail />} />
    </Routes>,
    { initialEntries: ["/profiles/default"], auth: { role } },
  );

const card = async () =>
  within(
    (await screen.findByText("IMS Filter Criteria")).closest(
      ".MuiCard-root",
    ) as HTMLElement,
  );

const editor = () =>
  screen.getByLabelText(/^Filter Criteria \(JSON\)/) as HTMLTextAreaElement;

const openEditor = async (user: ReturnType<typeof userEvent.setup>) => {
  await user.click(
    await screen.findByRole("button", { name: "Edit IMS filter criteria" }),
  );
  return within(await screen.findByRole("dialog"));
};

const putBody = () =>
  api.requests(IFC_PATH).find((r) => r.method === "PUT")?.body;

describe("ProfileDetail IMS filter criteria", () => {
  it("lists the profile's filter criteria in priority order", async () => {
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [
        { ...terminatingIFC, priority: 1 },
        { ...registerIFC, priority: 0 },
      ],
    }));
    renderProfile();

    const c = await card();
    await c.findByText("voicemail");
    const rows = c.getAllByRole("row").slice(1);
    expect(rows.map((r) => r.textContent)).toEqual([
      "1voicemailmethod REGISTERContinue",
      "2confsession case TERMINATING_UNREGISTEREDTerminate",
    ]);
  });

  it("shows when the profile has no filter criteria", async () => {
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    renderProfile();

    expect(
      await (await card()).findByText("No IMS filter criteria."),
    ).toBeInTheDocument();
  });

  it("saves edited JSON without priorities", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [{ ...registerIFC, priority: 0 }],
    }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    expect(JSON.parse(editor().value)).toEqual([registerIFC]);

    await user.clear(editor());
    await user.click(editor());
    await user.paste(JSON.stringify([terminatingIFC, registerIFC]));
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({
      initial_filter_criteria: [terminatingIFC, registerIFC],
    });
  });

  it("appends the MCPTT preset for the chosen application server", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [{ ...registerIFC, priority: 0 }],
    }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    expect(dialog.getByRole("button", { name: "Add preset" })).toBeDisabled();
    await user.click(dialog.getByRole("combobox", { name: /Preset/ }));
    await user.click(await screen.findByRole("option", { name: /^MCPTT/ }));
    await user.click(
      dialog.getByRole("combobox", { name: /Application Server/ }),
    );
    await user.click(
      await screen.findByRole("option", { name: /^mcptt \(sip:/ }),
    );
    await user.click(dialog.getByRole("button", { name: "Add preset" }));

    expect(JSON.parse(editor().value)).toEqual([
      registerIFC,
      ...mcpttPresetIFCs,
    ]);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({
      initial_filter_criteria: [registerIFC, ...mcpttPresetIFCs],
    });
  });

  it("clears the filter criteria with an empty array", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [{ ...registerIFC, priority: 0 }],
    }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    await user.clear(editor());
    await user.click(editor());
    await user.paste("[]");
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({ initial_filter_criteria: [] });
  });

  it("refuses invalid JSON and criteria without an application server", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    renderProfile();

    await (await card()).findByText("No IMS filter criteria.");
    const dialog = await openEditor(user);

    await user.clear(editor());
    await user.click(editor());
    await user.paste("[{");
    await user.tab();
    expect(await dialog.findByText("Must be valid JSON")).toBeInTheDocument();
    expect(dialog.getByRole("button", { name: "Save" })).toBeDisabled();

    await user.clear(editor());
    await user.paste('{"application_server_name": "mcptt"}');
    expect(await dialog.findByText("Must be a JSON array")).toBeInTheDocument();

    await user.clear(editor());
    await user.paste('[{"default_handling": "SESSION_CONTINUED"}]');
    expect(
      await dialog.findByText(
        "Filter criterion 1 needs an application_server_name",
      ),
    ).toBeInTheDocument();

    await user.clear(editor());
    await user.paste('[{"application_server_name": "  "}]');
    expect(
      await dialog.findByText(
        "Filter criterion 1 needs an application_server_name",
      ),
    ).toBeInTheDocument();
    expect(dialog.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(api.requests(IFC_PATH).some((r) => r.method === "PUT")).toBe(false);
  });

  it("shows the error when the server rejects the filter criteria", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    api.put(IFC_PATH, () =>
      httpError(400, "initial_filter_criteria[0]: unknown application server"),
    );
    renderProfile();

    await (await card()).findByText("No IMS filter criteria.");
    const dialog = await openEditor(user);
    await user.clear(editor());
    await user.click(editor());
    await user.paste('[{"application_server_name": "missing"}]');
    await user.click(dialog.getByRole("button", { name: "Save" }));

    expect(
      await dialog.findByText(
        "Failed to update IMS filter criteria: initial_filter_criteria[0]: unknown application server",
      ),
    ).toBeInTheDocument();
  });

  it("hides the edit button from a read-only user", async () => {
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [{ ...registerIFC, priority: 0 }],
    }));
    renderProfile("Read Only");

    await (await card()).findByText("voicemail");
    expect(
      screen.queryByRole("button", { name: "Edit IMS filter criteria" }),
    ).not.toBeInTheDocument();
  });
});
