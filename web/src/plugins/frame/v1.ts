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
  // Whether the page has asked which of its words are names: one that
  // hasn't drew none, and isn't told when what there is to name changes.
  let asked = false;

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
      case "ws:refs":
        asked = true;
        send({ type: "ws:refs-found", id: m.id, found: host.refs(m.texts) } satisfies V1.RefsFound);
        break;
      case "ws:hover":
        host.hover(m.ref !== undefined && m.box ? { ref: m.ref, box: m.box } : undefined);
        break;
      case "ws:here":
        host.here(m.name);
        break;
      case "ws:chrome-layout":
        host.chromeLayout(m.mode);
        break;
      // The conversation about the page: said to ws's own host, as ws's
      // own pages say it.
      case "ws:conversation": {
        const { type: _, ...settings } = m;
        host.conversation.set(settings);
        break;
      }
      case "ws:conversation-open": {
        const { type: _, ...opening } = m;
        host.conversation.open(opening);
        break;
      }
      case "ws:conversation-close":
        host.conversation.close();
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
    // ws finds its names in the app's words, when it's asked (ws:refs).
    refs: true,
    // ws holds conversations about the page (ws:conversation).
    conversations: true,
  } satisfies V1.Welcome);

  return {
    version: 1,
    navigate: (path) => send({ type: "ws:navigate", path } satisfies V1.Navigate),
    theme: () => send({ type: "ws:theme", ...host.theme() } satisfies V1.ThemeChanged),
    changed: () => send({ type: "ws:changed" } satisfies V1.Changed),
    refsChanged: () => {
      if (asked) send({ type: "ws:refs-changed" } satisfies V1.RefsChanged);
    },
    // Said to every page, asked for or not: one with nothing open does
    // nothing with it.
    outside: () => send({ type: "ws:outside" } satisfies V1.Outside),
    conversation: (s) => send({ type: "ws:conversation-state", open: s.open, ...(s.as ? { as: s.as } : {}), working: s.working } satisfies V1.ConversationState),
    close() {
      port.onmessage = null;
      port.close();
    },
  };
}
