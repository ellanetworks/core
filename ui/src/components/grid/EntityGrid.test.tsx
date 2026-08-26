// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import EntityGrid from "./EntityGrid";

const rows = [
  { id: 1, name: "charlie" },
  { id: 2, name: "alice" },
  { id: 3, name: "bob" },
];

const columns = [{ field: "name", headerName: "Name", flex: 1 }];

const names = () =>
  screen
    .getAllByRole("gridcell")
    .map((cell) => cell.textContent)
    .filter(Boolean);

describe("EntityGrid sorting", () => {
  it("sorts when the grid holds every row", async () => {
    render(<EntityGrid rows={rows} columns={columns} />);

    await userEvent.click(screen.getByRole("columnheader", { name: /name/i }));

    expect(names()).toEqual(["alice", "bob", "charlie"]);
  });

  it("offers no sorting when rows are paginated server-side", async () => {
    render(
      <EntityGrid
        rows={rows}
        columns={columns}
        paginationMode="server"
        rowCount={100}
        paginationModel={{ page: 0, pageSize: 25 }}
        onPaginationModelChange={() => {}}
      />,
    );

    await userEvent.click(screen.getByRole("columnheader", { name: /name/i }));

    expect(names()).toEqual(["charlie", "alice", "bob"]);
  });
});
