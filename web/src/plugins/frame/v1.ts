import { V1 } from "@withworkspace/ui-lib/protocol";
import { frameKeys } from "../../keys";
import type { Host, Session } from "../frame";
import { apiBase, frameBase, viewBase } from "../paths";

// Frame protocol 1's host: ws's side of it, once the handshake has chosen
// it. Every message it sends is checked against the protocol's own types
// (satisfies), and what the page sends is read with the protocol's guards:
// the same definition the page's client is built from, so the two can't
// drift apart unnoticed. A message this version doesn't know is dropped,
// which is what lets the version gain messages.
export function v1(port: MessagePort, host: Host): Session {
  const send = (m: V1.HostMessage) => port.postMessage(m);

  port.onmessage = (e: MessageEvent) => {
    const m: unknown = e.data;
    if (!V1.isPageMessage(m)) return;
    switch (m.type) {
      case "ws:go":
        host.go(m.path, m.replace ?? false);
        break;
      case "ws:title":
        host.titled(m.title);
        break;
      case "ws:key":
        host.key({ key: m.key, meta: m.meta, ctrl: m.ctrl, shift: m.shift, alt: m.alt });
        break;
    }
  };

  send({
    type: "ws:welcome",
    version: 1,
    ws: host.ws,
    plugin: { name: host.plugin, at: viewBase(host.plugin), base: frameBase(host.plugin), api: apiBase(host.plugin) },
    path: host.path(),
    theme: host.theme(),
    // Which keys are ws's is ws's to say (keys.ts, beside the shortcuts
    // themselves): the page passes on what it's told to, so a new shortcut
    // reaches every plugin with no new release of the package.
    keys: frameKeys,
  } satisfies V1.Welcome);

  return {
    version: 1,
    navigate: (path) => send({ type: "ws:navigate", path } satisfies V1.Navigate),
    theme: () => send({ type: "ws:theme", ...host.theme() } satisfies V1.ThemeChanged),
    changed: () => send({ type: "ws:changed" } satisfies V1.Changed),
    close() {
      port.onmessage = null;
      port.close();
    },
  };
}
