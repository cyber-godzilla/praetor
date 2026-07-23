import { describe, expect, it } from "vitest";
import { FixtureTransport } from "./transport-fixture";
import { Kind } from "./types";

describe("fixture transport", () => {
  it("drives a complete snapshot without Wails or a server", async () => {
    const transport = new FixtureTransport();
    let snapshotKinds: string[] = [];
    transport.subscribe({
      events: () => {},
      snapshot: (events) => (snapshotKinds = events.map((event) => event.kind)),
    });
    await transport.start();
    expect(snapshotKinds).toContain("conn");
    expect(snapshotKinds).toContain("text");
    expect(snapshotKinds).toContain("bars");
    expect(snapshotKinds).toContain("minimap");
    expect(snapshotKinds).toContain("compass");
    expect(snapshotKinds).toContain("notify");
    expect(new Set(snapshotKinds)).toEqual(new Set(Object.values(Kind)));
  });
});
