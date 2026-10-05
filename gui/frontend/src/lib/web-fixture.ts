import { Kind, type WireEvent } from "./types";

// Complete deterministic protocol fixture shared by transport/store tests and
// future browser screenshot tests. It deliberately contains every frontend
// WireEvent kind and a full current-state payload without any live TEC data.
export const webFixtureEvents: WireEvent[] = [
  { kind: Kind.Conn, conn: { state: "connected" } },
  {
    kind: Kind.Text,
    text: {
      text: "A bronze lamp glows on a stone table.",
      segments: [
        { text: "A " },
        { text: "bronze lamp", color: "#d88b45", bold: true },
        { text: " glows on a stone table." },
      ],
      timestamp: 1_721_350_800_000,
    },
  },
  {
    kind: Kind.Suppressed,
    suppressed: {
      channel: 1,
      sourceName: "Example",
      placeholderText: "[suppressed: Example OOC]",
      placeholderStyled: [{ text: "[suppressed: Example OOC]", color: "#888888" }],
      originalText: "(OOC) fixture text",
      originalStyled: [{ text: "(OOC) fixture text" }],
      timestamp: 1_721_350_801_000,
    },
  },
  { kind: Kind.Command, command: "look" },
  {
    kind: Kind.Status,
    status: {
      mode: "fixture",
      current: {
        mode: "fixture",
        start: 1_721_350_800_000,
        end: 0,
        durationMs: 1000,
        entries: [{ label: "Actions", value: 3 }],
      },
      history: [],
    },
  },
  {
    kind: Kind.Bars,
    bars: {
      hasHealth: true,
      health: 80,
      hasFatigue: true,
      fatigue: 60,
      hasEncumbrance: true,
      encumbrance: 40,
      hasSatiation: true,
      satiation: 20,
      hasLighting: true,
      lighting: 75,
      lightingRaw: 23,
    },
  },
  { kind: Kind.Minimap, image: { dataURI: "data:image/png;base64,fixture", width: 120, height: 120 } },
  { kind: Kind.Compass, image: { dataURI: "data:image/png;base64,fixture", width: 120, height: 120 } },
  { kind: Kind.Notify, notify: { title: "Fixture alert", message: "Notification delivery works." } },
  { kind: Kind.Error, error: { context: "fixture", error: "Example recoverable error" } },
  { kind: Kind.Debug, debug: { channel: 9, payload: "23" } },
  { kind: Kind.OpenMenu, openMenu: "help" },
];

export const webFixtureReconnect: WireEvent[] = [
  { kind: Kind.Conn, conn: { state: "disconnected", reason: "fixture disconnect" } },
  { kind: Kind.Conn, conn: { state: "connected" } },
];
