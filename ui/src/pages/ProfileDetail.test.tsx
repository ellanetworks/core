// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
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
  trigger: {
    conditions_match: "all",
    groups: [{ conditions: [{ method: "REGISTER" }] }],
  },
  default_handling: "SESSION_CONTINUED",
  include_register_request: false,
  include_register_response: false,
};

const terminatingIFC = {
  application_server_name: "conf",
  trigger: {
    conditions_match: "all",
    groups: [{ conditions: [{ session_case: "TERMINATING_UNREGISTERED" }] }],
  },
  default_handling: "SESSION_TERMINATED",
  include_register_request: false,
  include_register_response: false,
};

const mcpttPresetIFCs = [
  {
    application_server_name: "mcptt",
    trigger: {
      conditions_match: "all",
      groups: [{ conditions: [{ method: "REGISTER" }] }],
    },
    default_handling: "SESSION_CONTINUED",
    include_register_request: true,
    include_register_response: true,
  },
  {
    application_server_name: "mcptt",
    trigger: {
      conditions_match: "all",
      groups: [
        {
          conditions: [
            { session_case: "ORIGINATING_REGISTERED" },
            {
              sip_header: {
                header: "^P-Asserted-Service$",
                content: "^urn:urn-7:3gpp-service\\.ims\\.icsi\\.mcptt$",
              },
            },
          ],
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

const openEditor = async (user: ReturnType<typeof userEvent.setup>) => {
  await user.click(
    await screen.findByRole("button", { name: "Edit IMS filter criteria" }),
  );
  return within(
    await screen.findByRole("dialog", { name: "Edit IMS Filter Criteria" }),
  );
};

const criterionForm = async () =>
  within(await screen.findByRole("dialog", { name: /Filter Criterion$/ }));

const choose = async (
  user: ReturnType<typeof userEvent.setup>,
  scope: ReturnType<typeof within>,
  name: string | RegExp,
  option: string | RegExp,
) => {
  await user.click(scope.getByRole("combobox", { name }));
  await user.click(await screen.findByRole("option", { name: option }));
};

const putBody = () =>
  api.requests(IFC_PATH).find((r) => r.method === "PUT")?.body;

const rows = (dialog: ReturnType<typeof within>) =>
  dialog.queryAllByRole("listitem").map((r: HTMLElement) => r.textContent);

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
    const tableRows = c.getAllByRole("row").slice(1);
    expect(tableRows.map((r) => r.textContent)).toEqual([
      "1voicemailmethod is REGISTERContinue",
      "2confTerminating (unregistered)Terminate",
    ]);
  });

  it("shows when the profile has no filter criteria", async () => {
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    renderProfile();

    expect(
      await (await card()).findByText("No IMS filter criteria."),
    ).toBeInTheDocument();
  });

  it("saves the criteria unchanged without priorities", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [
        { ...registerIFC, priority: 0 },
        { ...terminatingIFC, priority: 1 },
      ],
    }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    expect(rows(dialog)).toEqual([
      "1voicemailmethod is REGISTERContinue",
      "2confTerminating (unregistered)Terminate",
    ]);

    await user.click(dialog.getByRole("button", { name: "Edit criterion 1" }));
    await user.click(
      (await criterionForm()).getByRole("button", { name: "Update" }),
    );
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({
      initial_filter_criteria: [registerIFC, terminatingIFC],
    });
  });

  it("edits a criterion's trigger and handling", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [{ ...registerIFC, priority: 0 }],
    }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    await user.click(dialog.getByRole("button", { name: "Edit criterion 1" }));
    const form = await criterionForm();

    const condition = within(form.getByRole("group", { name: "Condition 1" }));
    expect(condition.getByRole("combobox", { name: "Method" })).toHaveValue(
      "REGISTER",
    );
    await user.click(
      condition.getByRole("checkbox", { name: "Re-registration" }),
    );
    await user.click(
      form.getByRole("radio", { name: "Terminate the session" }),
    );
    await user.click(form.getByRole("button", { name: "Update" }));

    expect(rows(dialog)).toEqual([
      "1voicemailmethod is REGISTER (initial, de-registration)Terminate",
    ]);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({
      initial_filter_criteria: [
        {
          ...registerIFC,
          trigger: {
            conditions_match: "all",
            groups: [
              {
                conditions: [
                  {
                    method: "REGISTER",
                    registration_types: [
                      "INITIAL_REGISTRATION",
                      "DE-REGISTRATION",
                    ],
                  },
                ],
              },
            ],
          },
          default_handling: "SESSION_TERMINATED",
        },
      ],
    });
  });

  it("adds a criterion with condition groups", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("No IMS filter criteria.");
    const dialog = await openEditor(user);
    await user.click(dialog.getByRole("button", { name: "Add Criterion" }));
    const form = await criterionForm();

    await choose(user, form, "Application Server", /^voicemail/);
    await user.type(
      within(form.getByRole("group", { name: "Condition 1" })).getByRole(
        "combobox",
        { name: "Method" },
      ),
      "INVITE",
    );
    await user.click(form.getByRole("button", { name: "Add Condition" }));
    const second = within(form.getByRole("group", { name: "Condition 2" }));
    await choose(user, second, "Field", "SIP method");
    await user.type(
      second.getByRole("combobox", { name: "Method" }),
      "MESSAGE",
    );
    await choose(user, form, "Conditions match", "any");

    await user.click(form.getByRole("button", { name: "Add Group" }));
    expect(
      form.getByText(/The criterion matches when every group matches/),
    ).toBeInTheDocument();
    const third = within(
      form.getByRole("group", { name: "Condition 1 of group 2" }),
    );
    await choose(user, third, "Field", "SIP header");
    await choose(user, third, "Operator", "is absent");
    await user.type(
      third.getByRole("textbox", { name: "Header name (regex)" }),
      "^Privacy$",
    );
    await user.click(form.getByRole("button", { name: "Add" }));

    expect(rows(dialog)).toEqual([
      "1voicemail(method is INVITE or method is MESSAGE) and header ^Privacy$ absentContinue",
    ]);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({
      initial_filter_criteria: [
        {
          application_server_name: "voicemail",
          trigger: {
            conditions_match: "any",
            groups: [
              { conditions: [{ method: "INVITE" }, { method: "MESSAGE" }] },
              {
                conditions: [
                  { negated: true, sip_header: { header: "^Privacy$" } },
                ],
              },
            ],
          },
          default_handling: "SESSION_CONTINUED",
          include_register_request: false,
          include_register_response: false,
        },
      ],
    });
  });

  it("adds an unconditional criterion with advanced settings", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("No IMS filter criteria.");
    const dialog = await openEditor(user);
    await user.click(dialog.getByRole("button", { name: "Add Criterion" }));
    const form = await criterionForm();

    await user.click(form.getByRole("radio", { name: "Always" }));
    await user.click(form.getByRole("button", { name: "Advanced" }));
    await choose(user, form, "Applies To", "Unregistered subscribers");
    await user.type(form.getByRole("textbox", { name: "Service Info" }), "vm");
    await user.click(form.getByRole("button", { name: "Add" }));

    expect(rows(dialog)).toEqual(["1mcpttAlwaysContinue"]);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({
      initial_filter_criteria: [
        {
          application_server_name: "mcptt",
          default_handling: "SESSION_CONTINUED",
          profile_part: "UNREGISTERED",
          service_info: "vm",
          include_register_request: false,
          include_register_response: false,
        },
      ],
    });
  });

  it("refuses an incomplete condition", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    renderProfile();

    await (await card()).findByText("No IMS filter criteria.");
    const dialog = await openEditor(user);
    await user.click(dialog.getByRole("button", { name: "Add Criterion" }));
    const form = await criterionForm();
    const condition = within(form.getByRole("group", { name: "Condition 1" }));

    await user.click(condition.getByRole("combobox", { name: "Method" }));
    await user.tab();
    expect(await form.findByText("Enter a SIP method")).toBeInTheDocument();
    expect(form.getByRole("button", { name: "Add" })).toBeDisabled();

    await choose(user, condition, "Field", "Request-URI");
    await user.type(
      condition.getByRole("textbox", { name: "Pattern (regex)" }),
      "([[",
    );
    await user.tab();
    expect(
      await form.findByText("Must be a regular expression"),
    ).toBeInTheDocument();
    expect(form.getByRole("button", { name: "Add" })).toBeDisabled();

    await choose(user, condition, "Field", "SIP method");
    await user.type(
      condition.getByRole("combobox", { name: "Method" }),
      "REGISTER",
    );
    for (const name of [
      "Initial registration",
      "Re-registration",
      "De-registration",
    ])
      await user.click(condition.getByRole("checkbox", { name }));
    expect(await form.findByText("Select at least one")).toBeInTheDocument();
    expect(form.getByRole("button", { name: "Add" })).toBeDisabled();
  });

  it("warns about a negated session case", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    renderProfile();

    await (await card()).findByText("No IMS filter criteria.");
    const dialog = await openEditor(user);
    await user.click(dialog.getByRole("button", { name: "Add Criterion" }));
    const form = await criterionForm();
    const condition = within(form.getByRole("group", { name: "Condition 1" }));

    await choose(user, condition, "Field", "Session case");
    await choose(user, condition, "Operator", "is not");
    expect(
      form.getByText(/also matches session cases that later 3GPP releases add/),
    ).toBeInTheDocument();
  });

  it("deletes a criterion", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [
        { ...registerIFC, priority: 0 },
        { ...terminatingIFC, priority: 1 },
      ],
    }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    await user.click(
      dialog.getByRole("button", { name: "Delete criterion 1" }),
    );
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({ initial_filter_criteria: [terminatingIFC] });
  });

  it("reorders criteria by dragging", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [
        { ...registerIFC, priority: 0 },
        { ...terminatingIFC, priority: 1 },
      ],
    }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    const [first, second] = dialog.getAllByRole("listitem");
    const dataTransfer = {
      setDragImage: () => {},
      effectAllowed: "",
      dropEffect: "",
    };
    fireEvent.dragStart(second, { dataTransfer });
    fireEvent.dragOver(first, { dataTransfer });
    fireEvent.drop(first, { dataTransfer });
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({
      initial_filter_criteria: [terminatingIFC, registerIFC],
    });
  });

  it("appends the MCPTT preset once for the chosen application server", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [{ ...registerIFC, priority: 0 }],
    }));
    api.put(IFC_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    expect(dialog.getByRole("button", { name: "Add preset" })).toBeDisabled();
    await choose(user, dialog, /Preset/, /^MCPTT/);
    await choose(user, dialog, /Application Server/, /^mcptt \(sip:/);
    await user.click(dialog.getByRole("button", { name: "Add preset" }));

    expect(
      dialog.getByText("This preset is already added."),
    ).toBeInTheDocument();
    expect(dialog.getByRole("button", { name: "Add preset" })).toBeDisabled();
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS filter criteria updated successfully.");
    expect(putBody()).toEqual({
      initial_filter_criteria: [registerIFC, ...mcpttPresetIFCs],
    });
  });

  it("points to the Operator page when there is no application server", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({ initial_filter_criteria: [] }));
    api.get(AS_PATH, () => ({ items: [] }));
    renderProfile();

    await (await card()).findByText("No IMS filter criteria.");
    const dialog = await openEditor(user);
    expect(
      await dialog.findByRole("link", { name: "Operator page" }),
    ).toBeInTheDocument();
    expect(
      dialog.queryByRole("button", { name: "Add Criterion" }),
    ).not.toBeInTheDocument();
  });

  it("shows the error when the server rejects the filter criteria", async () => {
    const user = userEvent.setup();
    api.get(IFC_PATH, () => ({
      initial_filter_criteria: [{ ...registerIFC, priority: 0 }],
    }));
    api.put(IFC_PATH, () =>
      httpError(400, "Application server not found: voicemail"),
    );
    renderProfile();

    await (await card()).findByText("voicemail");
    const dialog = await openEditor(user);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    expect(
      await dialog.findByText(
        "Failed to update IMS filter criteria: Application server not found: voicemail",
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
