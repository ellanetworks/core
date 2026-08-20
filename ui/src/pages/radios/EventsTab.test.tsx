// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, beforeEach } from "vitest";
import { screen, waitFor, within, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import { radioEvent, radioEventPage } from "@/test/fixtures";
import type { APIRadioEvent } from "@/queries/radio_events";
import EventsTab from "./EventsTab";

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
        supported_tais: [],
      },
      {
        name: "radio-2",
        id: "2",
        address: "10.0.0.2",
        type: "gNB",
        supported_tais: [],
      },
    ],
    page: 1,
    per_page: 100,
    total_count: 2,
  }));
};

const renderEvents = async (path = "/radios/events") => {
  const result = renderWithProviders(<EventsTab />, {
    initialEntries: [path],
    auth: {},
  });
  await screen.findByRole("heading", { name: "Network Events" });
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

beforeEach(() => {
  seedApi();
});

describe("EventsTab filters authorization", () => {
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

describe("EventsTab filters", () => {
  it("omits every filter the operator has not set", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    const params = lastEventParams();

    expect(params).not.toHaveProperty("radio");
    expect(params).not.toHaveProperty("protocol");
    expect(params).not.toHaveProperty("direction");
    expect(params).not.toHaveProperty("message_type");
    expect(params).not.toHaveProperty("timestamp_from");
    expect(params).not.toHaveProperty("timestamp_to");
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

describe("EventsTab protocol and message type", () => {
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

describe("EventsTab timestamps", () => {
  it("sends the From bound as an ISO instant", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-01T10:30" },
    });

    await waitFor(() =>
      expect(lastEventParams().timestamp_from).toBe("2026-08-01T10:30:00.000Z"),
    );
  });

  it("survives a timestamp the browser cannot parse", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "999999-01-01T00:00" },
    });

    expect(
      await screen.findByRole("heading", { name: "Network Events" }),
    ).toBeVisible();
  });

  it("does not send an unparseable timestamp to the API", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "999999-01-01T00:00" },
    });
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Network Events" }),
      ).toBeVisible(),
    );

    const sent = eventRequests()
      .map((r) => r.params.get("timestamp_from"))
      .filter(Boolean);
    expect(sent.filter((v) => Number.isNaN(Date.parse(v!)))).toEqual([]);
  });

  it("rejects a To bound that precedes the From bound", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /to.*must be on or after.*from/i,
    );
  });
});

describe("EventsTab stale results", () => {
  const setInvalidRange = async () => {
    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });
    await screen.findByRole("alert");
  };

  it("stops showing event rows once the range is invalid", async () => {
    seedApi({ events: [radioEvent(1, { radio: "radio-7" })] });
    await renderEvents();
    await screen.findAllByText("radio-7");

    await setInvalidRange();

    expect(screen.queryByText("radio-7")).not.toBeInTheDocument();
  });

  it("shows no loading indicator for a request it will never send", async () => {
    seedApi({ events: [radioEvent(1, { radio: "radio-7" })] });
    await renderEvents();
    await screen.findAllByText("radio-7");

    await setInvalidRange();

    expect(screen.queryAllByRole("progressbar")).toEqual([]);
    expect(document.querySelectorAll(".MuiLinearProgress-root")).toHaveLength(
      0,
    );
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

describe("EventsTab timestamp accessibility", () => {
  const invert = async () => {
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

  it("carries no stale description once the range is valid", async () => {
    await renderEvents();
    await waitForEventRequests(1);
    await invert();

    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-20T10:00" },
    });

    await waitFor(() =>
      expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
    );
    expect(screen.getByLabelText("To")).toHaveAccessibleDescription("");
  });

  it("stops the picker offering a To before the From", async () => {
    await renderEvents();
    await waitForEventRequests(1);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });

    await waitFor(() =>
      expect(screen.getByLabelText("To")).toHaveAttribute(
        "min",
        "2026-08-10T10:00",
      ),
    );
  });
});

describe("EventsTab pagination", () => {
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

describe("EventsTab table", () => {
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
