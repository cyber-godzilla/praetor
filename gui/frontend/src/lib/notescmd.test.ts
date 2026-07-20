import { describe, it, expect } from "vitest";
import { parseNotesCommand, formatNotesList } from "./notescmd";

describe("parseNotesCommand", () => {
  it("bare /notes opens the list", () => {
    expect(parseNotesCommand("")).toEqual({ kind: "open-list" });
    expect(parseNotesCommand("   ")).toEqual({ kind: "open-list" });
  });
  it("add with and without a title", () => {
    expect(parseNotesCommand("add Combat Plan")).toEqual({ kind: "new", title: "Combat Plan" });
    expect(parseNotesCommand("add")).toEqual({ kind: "new", title: "" });
  });
  it("open requires a title", () => {
    expect(parseNotesCommand("open Combat Plan")).toEqual({ kind: "open", title: "Combat Plan" });
    expect(parseNotesCommand("open")).toEqual({ kind: "usage" });
  });
  it("delete requires a title", () => {
    expect(parseNotesCommand("delete Old")).toEqual({ kind: "delete", title: "Old" });
    expect(parseNotesCommand("delete")).toEqual({ kind: "usage" });
  });
  it("list", () => {
    expect(parseNotesCommand("list")).toEqual({ kind: "list" });
  });
  it("subcommand is case-insensitive; title keeps case", () => {
    expect(parseNotesCommand("OPEN MyNote")).toEqual({ kind: "open", title: "MyNote" });
  });
  it("unknown subcommand is usage", () => {
    expect(parseNotesCommand("frob x")).toEqual({ kind: "usage" });
  });
});

describe("formatNotesList", () => {
  it("empty", () => {
    expect(formatNotesList([])).toEqual(["No notes."]);
  });
  it("title with preview, and title-only", () => {
    expect(
      formatNotesList([
        { title: "A", preview: "hello" },
        { title: "B", preview: "" },
      ]),
    ).toEqual(["A — hello", "B"]);
  });
});
