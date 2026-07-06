// Ambient declarations for the Wails-injected globals. Wails generates
// window.go.<pkg>.<Struct>.<Method> bindings and a window.runtime with the
// events API. We declare only what we use so the app builds without the
// generated wailsjs/ directory (which is produced at `wails dev`/`build` time).

export {};

declare global {
  interface Window {
    go?: {
      gui?: {
        GuiApp?: Record<string, (...args: any[]) => Promise<any>>;
      };
    };
    runtime?: {
      EventsOn: (event: string, cb: (data: any) => void) => () => void;
      EventsOff: (event: string, ...additional: string[]) => void;
      EventsEmit: (event: string, ...data: any[]) => void;
      Quit: () => void;
      WindowMinimise: () => void;
      WindowToggleMaximise: () => void;
    };
  }
}
