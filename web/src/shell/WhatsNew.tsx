import { useEffect, useState, useSyncExternalStore } from "react";
import { useNavigate } from "react-router";
import { getWhatsNew, seenWhatsNew, type WhatsNewEntry, type WhatsNewInfo } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Icon } from "../ui/Icon";
import { Dialog } from "../ui/Dialog";
import { Logotype } from "../ui/Logotype";
import { shortVersion } from "./version";
import { Button } from "../ui/Button";

// What's new: after ws is upgraded, a dialog over whatever page is open
// says what's new in the releases since the one last seen in this
// workspace, once. The daemon keeps which that was, so it's once for
// every browser. Closing it, however it's closed, is having seen it.
// Settings' About opens it again (openWhatsNew), with everything this ws
// says is new.

let asked = false;
const listeners = new Set<() => void>();

// openWhatsNew shows everything this ws says is new.
export function openWhatsNew() {
  asked = true;
  listeners.forEach((l) => l());
}

function closeAsked() {
  asked = false;
  listeners.forEach((l) => l());
}

// WhatsNew is mounted once, in the shell: it asks when the ws serving the
// page changes (a daemon restarted onto an upgrade), and shows what's
// unseen.
export function WhatsNew({ version }: { version?: string }) {
  const [info, setInfo] = useState<WhatsNewInfo>();
  const [auto, setAuto] = useState(false);
  const open = useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => asked,
  );
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
    return <WhatsNewDialog title="What's new" entries={info.unseen} info={info} onClose={close} />;
  }
  if (open) {
    return <WhatsNewDialog title="What's new" entries={info.entries} info={{ ...info, missing: false }} onClose={closeAsked} />;
  }
  return null;
}

export function WhatsNewDialog({ title, entries, info, onClose }: { title: string; entries: WhatsNewEntry[]; info: WhatsNewInfo; onClose: () => void }) {
  const navigate = useNavigate();
  const releases = new Set(entries.map((e) => e.version ?? ""));
  // Each says its release only where there's more than one.
  const several = releases.size > 1;
  const [lead, ...rest] = entries;
  const show = (link: string) => {
    onClose();
    void navigate(link);
  };
  const things = entries.length === 1 ? "one thing" : `${entries.length} things`;
  const since = info.seen && info.unseen.length > 0 && entries === info.unseen ? `Since ${shortVersion(info.seen)}` : undefined;
  return (
    <Dialog title={title} onClose={onClose} bare className="whats-new">
      <header className="whats-new-head">
        <div className="whats-new-eyebrow">
          <Logotype height={22} />
          <span className="whats-new-version mono">{shortVersion(info.version)}</span>
        </div>
        <h2>{title}</h2>
        <p className="dim">
          {!info.release
            ? "A build of a clone: everything it says is new, with what's in no release yet, as it'll read."
            : [since, entries.length ? `${things}${several ? `, across ${releases.size} releases` : ""}` : undefined].filter(Boolean).join(" · ")}
        </p>
      </header>
      <div className="whats-new-body">
        {!lead ? (
          <p className="dim">Nothing's said to be new in this ws.</p>
        ) : (
          <>
            <article className="whats-new-lead">
              {lead.picture && <Picture src={lead.picture} alt={lead.title} />}
              <div className="whats-new-words">
                {several && <Release entry={lead} />}
                <h3>{lead.title}</h3>
                <Markdown text={lead.body} />
                {lead.link && <ShowMe onClick={() => show(lead.link!)} />}
              </div>
            </article>
            {rest.length > 0 && (
              <div className="whats-new-grid">
                {rest.map((e) => (
                  <article key={e.slug} className="whats-new-card">
                    {e.picture && <Picture src={e.picture} alt={e.title} />}
                    <div className="whats-new-words">
                      {several && <Release entry={e} />}
                      <h3>{e.title}</h3>
                      <Markdown text={e.body} />
                      {e.link && <ShowMe onClick={() => show(e.link!)} />}
                    </div>
                  </article>
                ))}
              </div>
            )}
          </>
        )}
      </div>
      <footer className="whats-new-foot">
        <span className="dim">
          {info.missing && info.releases && (
            <>
              There's more from further back than this ws keeps: it's on{" "}
              <a className="link" href={info.releases} target="_blank" rel="noopener noreferrer">
                the releases page
              </a>
              .
            </>
          )}
        </span>
        <Button variant="primary" onClick={onClose} autoFocus>
          Got it
        </Button>
      </footer>
    </Dialog>
  );
}

function Release({ entry }: { entry: WhatsNewEntry }) {
  return <span className="whats-new-release mono">{entry.version ? shortVersion(entry.version) : "Not released yet"}</span>;
}

function ShowMe({ onClick }: { onClick: () => void }) {
  return (
    <Button variant="ghost" className="whats-new-link" onClick={onClick}>
      Show me <Icon name="arrow-right" size={12} />
    </Button>
  );
}

// Picture is an entry's, fetched from its release by the daemon: one that
// can't be had (offline, not published) isn't there, and the words say
// it.
function Picture({ src, alt }: { src: string; alt: string }) {
  const [failed, setFailed] = useState(false);
  if (failed) return null;
  return <img className="whats-new-picture" src={src} alt={alt} loading="lazy" onError={() => setFailed(true)} />;
}
