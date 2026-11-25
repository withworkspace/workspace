import { useEffect, useState } from "react";
import { getWhatsNew, seenWhatsNew, type WhatsNewInfo } from "../../api";
import { closeAsked, useAsked } from "./asked";
import { News } from "./News";

// What's new: after ws is upgraded, a dialog over whatever page is open
// says what's new in the releases since the one last seen in this
// workspace, once. The daemon keeps which that was, so it's once for
// every browser. Closing it, however it's closed, is having seen it.
// Settings' About opens it again (openWhatsNew), with everything this ws
// says is new.

// WhatsNew is mounted once, in the shell: it asks when the ws serving the
// page changes (a daemon restarted onto an upgrade), and shows what's
// unseen.
export function WhatsNew({ version }: { version?: string }) {
  const [info, setInfo] = useState<WhatsNewInfo>();
  const [auto, setAuto] = useState(false);
  const open = useAsked();
  useEffect(() => {
    if (!version) return;
    let live = true;
    getWhatsNew()
      .then((n) => {
        if (!live) return;
        setInfo(n);
        if (n.unseen.length > 0) setAuto(true);
      })
      .catch(() => {}); // nothing's said: it's asked again on the next ws
    return () => {
      live = false;
    };
  }, [version]);
  // Asked for from About, it's asked again: seen may have changed since.
  useEffect(() => {
    if (open) void getWhatsNew().then(setInfo, () => {});
  }, [open]);

  if (!info) return null;
  if (auto) {
    const close = () => {
      setAuto(false);
      setInfo({ ...info, unseen: [] });
      void seenWhatsNew().catch(() => {}); // said again next time, at worst
    };
    return <News title="What's new" entries={info.unseen} info={info} onClose={close} />;
  }
  if (open) {
    return <News title="What's new" entries={info.entries} info={{ ...info, missing: false }} onClose={closeAsked} />;
  }
  return null;
}
