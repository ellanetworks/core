// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, beforeEach } from "vitest";
import { screen, waitFor, within, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import { radioEvent, radioEventPage } from "@/test/fixtures";
import type { APIRadioEvent } from "@/queries/radio_events";
import RadioEvents from "./RadioEvents";

const api = setupApiServer();

const EVENTS_PATH = "/api/v1/ran/events";
const NGAP_ONLY_MESSAGE = "AMFStatusIndication";
const S1AP_ONLY_MESSAGE = "S1SetupRequest";

const seedApi = ({
  events = [radioEvent(1)],
  totalCount,
}: { events?: APIRadioEvent[]; totalCount?: number } = {}) => {
  api.get(EVENTS_PATH, () =>
    radioEventPage(events, { total_count: totalCount ?? events.length }),
  );
  api.get("/api/v1/ran/events/retention", () => ({ days: 30 }));
  api.get("/api/v1/ran/radios", () => ({
    items: [
      {
        name: "radio-1",
        id: "1",
        address: "10.0.0.1",
        type: "gNB",
      },
      {
        name: "radio-2",
        id: "2",
        address: "10.0.0.2",
        type: "gNB",
      },
    ],
    page: 1,
    per_page: 100,
    total_count: 2,
  }));
};

const renderEvents = async (path = "/radios/events") => {
  const result = renderWithProviders(<RadioEvents />, {
    initialEntries: [path],
    auth: {},
  });
  await screen.findByRole("heading", { name: /Network Events/ });
  return result;
};

const eventRequests = () => api.requests(EVENTS_PATH);

const lastEventParams = () => {
  const request = api.lastRequest(EVENTS_PATH);
  if (!request) throw new Error("no ran/events request was made");
  return Object.fromEntries(request.params);
};

const waitForEventRequests = (count: number) =>
  waitFor(() => expect(eventRequests().length).toBeGreaterThanOrEqual(count));

const selectOption = async (
  user: ReturnType<typeof userEvent.setup>,
  label: string,
  option: string | RegExp,
) => {
  await user.click(screen.getByRole("combobox", { name: label }));
  await user.click(
    within(await screen.findByRole("listbox")).getByRole("option", {
      name: option,
    }),
  );
};

const timeRangeButton = () =>
  screen.getByRole("button", { name: /^Time range:/ });

const showCustomRange = async () => {
  fireEvent.click(timeRangeButton());
  await screen.findByLabelText("From");
};

const closeTimeRange = async () => {
  fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
  await waitFor(() =>
    expect(screen.queryByRole("menu")).not.toBeInTheDocument(),
  );
};

const selectQuickRange = async (
  user: ReturnType<typeof userEvent.setup>,
  name: string,
) => {
  await user.click(timeRangeButton());
  await user.click(await screen.findByRole("menuitem", { name }));
};

beforeEach(() => {
  seedApi();
});

describe("RadioEvents filters authorization", () => {
  it("sends the access token on every request", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    const requests = api.requests();
    expect(requests.length).toBeGreaterThan(0);
    for (const request of requests) {
      expect(
        request.headers.get("authorization"),
        `${request.method} ${request.url.pathname} was sent unauthenticated`,
      ).toBe("Bearer test-token");
    }
  });
});

describe("RadioEvents filters", () => {
  it("omits every filter the operator has not set", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    const params = lastEventParams();

    expect(params).not.toHaveProperty("radio");
    expect(params).not.toHaveProperty("protocol");
    expect(params).not.toHaveProperty("direction");
    expect(params).not.toHaveProperty("message_type");
    expect(params).not.toHaveProperty("start");
    expect(params).not.toHaveProperty("end");
  });

  it("sends the selected radio", async () => {
    const user = userEvent.setup();
    await renderEvents();
    await waitForEventRequests(1);

    await selectOption(user, "Radio", /radio-2/);

    await waitFor(() => expect(lastEventParams().radio).toBe("radio-2"));
  });

  it("sends the selected protocol", async () => {
    const user = userEvent.setup();
    await renderEvents();
    await waitForEventRequests(1);

    await selectOption(user, "Protocol", "NGAP (5G)");

    await waitFor(() => expect(lastEventParams().protocol).toBe("NGAP"));
  });

  it("sends the selected direction", async () => {
    const user = userEvent.setup();
    await renderEvents();
    await waitForEventRequests(1);

    await selectOption(user, "Direction", /Inbound/);

    await waitFor(() => expect(lastEventParams().direction).toBe("inbound"));
  });

  it("seeds the radio filter from the URL", async () => {
    await renderEvents("/radios/events?radio=radio-2");
    await waitForEventRequests(1);

    expect(lastEventParams().radio).toBe("radio-2");
  });
});

describe("RadioEvents protocol and message type", () => {
  it("sends a message type chosen under its own protocol", async () => {
    const user = userEvent.setup();
    await renderEvents();
    await waitForEventRequests(1);

    await selectOption(user, "Protocol", "NGAP (5G)");
    await selectOption(user, "Message Type", NGAP_ONLY_MESSAGE);

    await waitFor(() =>
      expect(lastEventParams().message_type).toBe(NGAP_ONLY_MESSAGE),
    );
  });

  it("never queries a message type the selected protocol cannot carry", async () => {
    const user = userEvent.setup();
    await renderEvents();
    await waitForEventRequests(1);

    await selectOption(user, "Protocol", "NGAP (5G)");
    await selectOption(user, "Message Type", NGAP_ONLY_MESSAGE);
    await waitFor(() =>
      expect(lastEventParams().message_type).toBe(NGAP_ONLY_MESSAGE),
    );

    await selectOption(user, "Protocol", "S1AP (4G)");

    await waitFor(() => expect(lastEventParams().protocol).toBe("S1AP"));
    const impossible = eventRequests().filter(
      (r) =>
        r.params.get("protocol") === "S1AP" &&
        r.params.get("message_type") === NGAP_ONLY_MESSAGE,
    );
    expect(impossible).toEqual([]);
  });

  it("does not restore a message type when its protocol is reselected", async () => {
    const user = userEvent.setup();
    await renderEvents();
    const messageType = () =>
      screen.getByRole("combobox", { name: "Message Type" });

    await selectOption(user, "Protocol", "NGAP (5G)");
    await selectOption(user, "Message Type", NGAP_ONLY_MESSAGE);
    await selectOption(user, "Protocol", "S1AP (4G)");
    await selectOption(user, "Protocol", "NGAP (5G)");

    expect(messageType()).toHaveValue("");
  });

  it("keeps a message type both protocols share", async () => {
    const user = userEvent.setup();
    await renderEvents();
    await waitForEventRequests(1);

    await selectOption(user, "Protocol", "S1AP (4G)");
    await selectOption(user, "Message Type", S1AP_ONLY_MESSAGE);
    await waitFor(() =>
      expect(lastEventParams().message_type).toBe(S1AP_ONLY_MESSAGE),
    );

    await selectOption(user, "Protocol", "S1AP (4G)");

    expect(lastEventParams().message_type).toBe(S1AP_ONLY_MESSAGE);
  });
});

describe("RadioEvents timestamps", () => {
  it("sends the From bound as an ISO instant", async () => {
    await renderEvents();
    await waitForEventRequests(1);
    await showCustomRange();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-01T10:30" },
    });

    await waitFor(() =>
      expect(lastEventParams().start).toBe("2026-08-01T10:30:00.000Z"),
    );
  });

  it("survives a timestamp the browser cannot parse", async () => {
    await renderEvents();
    await waitForEventRequests(1);
    await showCustomRange();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "999999-01-01T00:00" },
    });
    await closeTimeRange();

    expect(
      await screen.findByRole("heading", { name: /Network Events/ }),
    ).toBeVisible();
  });

  it("does not send an unparseable timestamp to the API", async () => {
    await renderEvents();
    await waitForEventRequests(1);
    await showCustomRange();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "999999-01-01T00:00" },
    });
    await closeTimeRange();
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: /Network Events/ }),
      ).toBeVisible(),
    );

    const sent = eventRequests()
      .map((r) => r.params.get("start"))
      .filter(Boolean);
    expect(sent.filter((v) => Number.isNaN(Date.parse(v!)))).toEqual([]);
  });
});

describe("RadioEvents stale results", () => {
  const setInvalidRange = async () => {
    await showCustomRange();
    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });
    await screen.findByRole("alert");
  };

  it("sends no request for the invalid range", async () => {
    seedApi({ events: [radioEvent(1, { radio: "radio-7" })] });
    await renderEvents();
    await screen.findAllByText("radio-7");

    await setInvalidRange();

    const inverted = eventRequests().filter((r) => {
      const from = r.params.get("start");
      const to = r.params.get("end");
      return !!from && !!to && from > to;
    });
    expect(inverted).toEqual([]);
  });

  it("restores the rows once the range is valid again", async () => {
    seedApi({ events: [radioEvent(1, { radio: "radio-7" })] });
    await renderEvents();
    await screen.findAllByText("radio-7");
    await setInvalidRange();

    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-20T10:00" },
    });

    expect((await screen.findAllByText("radio-7")).length).toBeGreaterThan(0);
  });
});

describe("RadioEvents timestamp accessibility", () => {
  const invert = async () => {
    await showCustomRange();
    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });
    await screen.findByRole("alert");
  };

  it.each(["From", "To"])(
    "describes the %s field with the reason it is invalid",
    async (label) => {
      await renderEvents();
      await waitForEventRequests(1);

      await invert();

      expect(screen.getByLabelText(label)).toHaveAccessibleDescription(
        /must be on or after/i,
      );
    },
  );
});

describe("RadioEvents relative time range", () => {
  it("resolves a relative range into a sliding lower bound", async () => {
    const user = userEvent.setup();
    await renderEvents();
    await waitForEventRequests(1);

    await selectQuickRange(user, "Last 15 minutes");

    await waitFor(() => expect(lastEventParams().start).toBeDefined());
    const params = lastEventParams();
    expect(params).not.toHaveProperty("relative_range");
    expect(params).not.toHaveProperty("end");
    expect(
      Math.abs(Date.parse(params.start) - (Date.now() - 15 * 60_000)),
    ).toBeLessThan(60_000);
  });

  it("offers the quick ranges and the custom bounds in one panel", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    expect(screen.queryByLabelText("From")).not.toBeInTheDocument();

    fireEvent.click(timeRangeButton());

    expect(await screen.findByLabelText("From")).toBeVisible();
    expect(screen.getByLabelText("To")).toBeVisible();
    expect(screen.getByRole("menuitem", { name: "Any time" })).toBeVisible();
    expect(
      screen.getByRole("menuitem", { name: "Last 6 hours" }),
    ).toBeVisible();
  });

  it("names the selected range on the button and closes the panel", async () => {
    const user = userEvent.setup();
    await renderEvents();
    await waitForEventRequests(1);

    await selectQuickRange(user, "Last 1 hour");

    await waitFor(() =>
      expect(screen.queryByRole("menuitem")).not.toBeInTheDocument(),
    );
    expect(timeRangeButton()).toHaveAccessibleName("Time range: Last 1 hour");
  });

  it("switches to the custom range when a bound is typed", async () => {
    await renderEvents();
    await waitForEventRequests(1);
    await showCustomRange();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-01T10:30" },
    });

    await waitFor(() =>
      expect(lastEventParams().start).toBe("2026-08-01T10:30:00.000Z"),
    );
    await closeTimeRange();
    expect(timeRangeButton()).toHaveAccessibleName(/After/);
  });
});

describe("RadioEvents pagination", () => {
  const manyEvents = Array.from({ length: 25 }, (_, i) => radioEvent(i + 1));

  it("returns to the first page when a filter changes", async () => {
    const user = userEvent.setup();
    seedApi({ events: manyEvents, totalCount: 500 });
    await renderEvents();
    await waitForEventRequests(1);

    await user.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() => expect(lastEventParams().page).toBe("2"));

    await selectOption(user, "Protocol", "NGAP (5G)");

    await waitFor(() => {
      const params = lastEventParams();
      expect(params.protocol).toBe("NGAP");
      expect(params.page).toBe("1");
    });
  });
});

describe("RadioEvents table", () => {
  it("renders an event row", async () => {
    seedApi({ events: [radioEvent(1, { radio: "radio-7" })] });
    await renderEvents();

    expect(await screen.findByText("radio-7")).toBeVisible();
  });

  it("distinguishes no-results from no-data", async () => {
    const user = userEvent.setup();
    seedApi({ events: [] });
    await renderEvents();

    expect(await screen.findByText("No radio events yet")).toBeVisible();

    await selectOption(user, "Protocol", "NGAP (5G)");

    expect(
      await screen.findByText("No radio events match the selected filters"),
    ).toBeVisible();
  });
});

const seedDecodedEvent = (id: number) =>
  api.get(`/api/v1/ran/events/${id}`, () => ({
    decoded: {
      pdu_type: "InitiatingMessage",
      procedure_code: { value: 0, name: "PathSwitchRequest" },
      criticality: { value: 0, name: "reject" },
      value: {},
    },
    raw: "00",
  }));

const panelTitle = () => screen.findByTestId("event-panel-title");

const panelIsOpen = async () =>
  (await screen.findByTestId("event-panel")).getAttribute("data-open") ===
  "true";

describe("RadioEvents event panel", () => {
  it("opens the panel for the event named in the URL", async () => {
    seedApi({
      events: [radioEvent(7, { message_type: "PathSwitchRequest" })],
    });
    seedDecodedEvent(7);
    await renderEvents("/radios/events?event=7");

    expect(await panelTitle()).toHaveTextContent("PathSwitchRequest");
    expect(await panelIsOpen()).toBe(true);
  });

  it("leaves the panel shut when the URL names no event", async () => {
    seedApi({
      events: [radioEvent(7, { message_type: "PathSwitchRequest" })],
    });
    await renderEvents();

    expect(await panelIsOpen()).toBe(false);
  });

  it("opens the panel for a row the operator clicks", async () => {
    const user = userEvent.setup();
    seedApi({
      events: [radioEvent(7, { message_type: "PathSwitchRequest" })],
    });
    seedDecodedEvent(7);
    await renderEvents();

    const grid = await screen.findByRole("grid");
    await user.click(await within(grid).findByText("PathSwitchRequest"));

    await waitFor(async () =>
      expect(await panelTitle()).toHaveTextContent("PathSwitchRequest"),
    );
  });

  it("keeps the panel shut for an event id that is not on the page", async () => {
    seedApi({
      events: [radioEvent(7, { message_type: "PathSwitchRequest" })],
    });
    await renderEvents("/radios/events?event=999");

    await screen.findByText("radio-1");
    expect(await panelIsOpen()).toBe(false);
  });

  it("keeps the panel open when the operator pages past the event", async () => {
    const user = userEvent.setup();
    seedApi({
      events: [radioEvent(7, { message_type: "PathSwitchRequest" })],
      totalCount: 60,
    });
    seedDecodedEvent(7);
    await renderEvents("/radios/events?event=7");
    expect(await panelIsOpen()).toBe(true);

    seedApi({ events: [radioEvent(42, { message_type: "Paging" })] });
    await user.click(screen.getByRole("button", { name: /next page/i }));

    await screen.findByText("Paging");
    expect(await panelIsOpen()).toBe(true);
  });

  it("shuts the panel again when the event is dismissed", async () => {
    const user = userEvent.setup();
    seedApi({
      events: [radioEvent(7, { message_type: "PathSwitchRequest" })],
    });
    seedDecodedEvent(7);
    await renderEvents("/radios/events?event=7");

    expect(await panelIsOpen()).toBe(true);

    await user.keyboard("{Escape}");

    await waitFor(async () => expect(await panelIsOpen()).toBe(false));
    expect(await panelTitle()).toHaveTextContent("PathSwitchRequest");
  });
});
