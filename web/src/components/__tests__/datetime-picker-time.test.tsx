/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";

import { DateTimePicker } from "../datetime-picker";

test("shows the seconds of the selected time", async () => {
  render(
    <DateTimePicker
      value={new Date(2026, 9, 11, 8, 30, 45)}
      onChange={() => undefined}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: /2026-10-11/ }));
  expect(await screen.findByLabelText("Time")).toHaveValue("08:30:45");
});

test("keeps the seconds typed into the time field", async () => {
  const onChange = vi.fn();
  render(
    <DateTimePicker
      value={new Date(2026, 9, 11, 8, 0, 0)}
      onChange={onChange}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: /2026-10-11/ }));
  fireEvent.change(await screen.findByLabelText("Time"), {
    target: { value: "08:30:45" },
  });
  expect(onChange).toHaveBeenLastCalledWith(new Date(2026, 9, 11, 8, 30, 45));
});

test("reads a time without seconds as zero seconds", async () => {
  const onChange = vi.fn();
  render(
    <DateTimePicker
      value={new Date(2026, 9, 11, 8, 0, 30)}
      onChange={onChange}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: /2026-10-11/ }));
  fireEvent.change(await screen.findByLabelText("Time"), {
    target: { value: "09:15" },
  });
  expect(onChange).toHaveBeenLastCalledWith(new Date(2026, 9, 11, 9, 15, 0));
});

test("keeps the current time when the time field is emptied", async () => {
  const onChange = vi.fn();
  render(
    <DateTimePicker
      value={new Date(2026, 9, 11, 20, 30, 45)}
      onChange={onChange}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: /2026-10-11/ }));
  const field = await screen.findByLabelText("Time");
  fireEvent.change(field, { target: { value: "" } });
  expect(onChange).not.toHaveBeenCalled();
  expect(field).toHaveValue("20:30:45");
});

test("clamps a seconds-level time to the configured minimum", async () => {
  const onChange = vi.fn();
  const minimum = new Date(2026, 9, 11, 8, 30, 45);
  render(
    <DateTimePicker value={minimum} minDate={minimum} onChange={onChange} />,
  );
  fireEvent.click(screen.getByRole("button", { name: /2026-10-11/ }));
  const field = await screen.findByLabelText("Time");
  fireEvent.change(field, { target: { value: "08:30:44" } });
  expect(onChange).toHaveBeenLastCalledWith(minimum);
  expect(field).toHaveValue("08:30:45");
});
