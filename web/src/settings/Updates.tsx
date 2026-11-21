import { useEffect, useState } from "react";
import { Link } from "react-router";
import { checkUpdates, getUpdates, type UpdatesInfo, upgradeWS } from "../api";
import { useLoaded } from "../load";
import { shortVersion } from "../shell/version";
import { Icon } from "../ui/Icon";
import { Callout, confirm, Copyable, ErrorCard, humanError, Skeleton, toast } from "../ui/kit";
import { Time } from "../ui/time";
import { Button } from "../ui/Button";

// Updates, on Settings' About: the latest release of ws as the daemon
// last found it (it looks once a day, and when Check now is pressed),
// against the one that's running. A later one is installed from here, as
// ws upgrade does it, where this ws is a release's build; the daemon
// restarts onto it by itself once nothing's at work.

// standing is where ws stands against its latest release, for the line
// that says so.
export type Standing = "waiting" | "out" | "out-built" | "latest" | "ahead" | "unknown";

export function standingOf(up: Pick<UpdatesInfo, "installed" | "newer" | "release" | "latest" | "version">): Standing {
  if (up.installed) return "waiting";
  if (up.newer) return up.release ? "out" : "out-built";
  if (!up.latest) return "unknown";
  return up.latest === up.version ? "latest" : "ahead";
}

export function Updates({ running }: { running?: string }) {
  // The daemon restarting onto another ws is this page's to hear of: what
  // it says of updates is asked again then.
  const { value: up, error, set, reload } = useLoaded<UpdatesInfo>(getUpdates, [running], { keep: true });
  const [busy, setBusy] = useState<"look" | "upgrade">();
  const waiting = !!up?.installed;
  // While a ws waits to be restarted onto, what it's waiting for changes.
  useEffect(() => {
    if (!waiting) return;
    const t = setInterval(() => {
      if (document.visibilityState === "visible") void reload();
    }, 5000);
    return () => clearInterval(t);
  }, [waiting, reload]);

  const look = () => {
    setBusy("look");
    checkUpdates()
      .then((now) => {
        set(now);
        if (now.error) toast(`Couldn't look: ${humanError(now.error)}`, { tone: "danger", ms: 8000 });
        else toast(now.newer ? `ws ${now.latest} is out` : "No later release of ws", { tone: "done" });
      })
      .catch((e: Error) => toast(`Couldn't look: ${humanError(e.message)}`, { tone: "danger", ms: 8000 }))
      .finally(() => setBusy(undefined));
  };
  const upgrade = async () => {
    if (!up?.latest) return;
    const yes = await confirm({
      title: `Upgrade ws to ${up.latest}?`,
      body: `It's fetched, checked against the release's checksums, and put in place of ${shortVersion(up.version)}, which is kept: ws upgrade --rollback puts it back. The daemon restarts onto it once nothing's at work (no crew mid-turn, no agent's run), and this page carries on.`,
      confirm: "Upgrade",
    });
    if (!yes) return;
    setBusy("upgrade");
    try {
      const did = await upgradeWS();
      toast(did.installed ? `ws ${did.to} is installed: the daemon restarts onto it once nothing's at work` : `ws ${did.to} is what's running already`, { tone: "done", ms: 8000 });
      await reload();
    } catch (e) {
      toast(`Couldn't upgrade: ${humanError((e as Error).message)}`, { tone: "danger", ms: 10000 });
    } finally {
      setBusy(undefined);
    }
  };

  if (error && !up) return <ErrorCard error={error} retry={reload} />;
  if (!up) return <Skeleton rows={2} />;
  const standing = standingOf(up);
  const notes = up.page && (
    <a href={up.page} target="_blank" rel="noopener noreferrer" className="link settings-update-notes">
      What's in it
      <Icon name="arrow-up-right" size={11} />
    </a>
  );
  return (
    <div className="settings-updates">
      {standing === "waiting" && (
        <Callout tone="info" icon="rotate" title={`ws ${up.installed} is installed`}>
          The daemon restarts onto it once nothing's at work
          {up.waits?.length ? (
            <>
              . It's waiting for:
              <ul className="settings-update-waits">
                {up.waits.map((w) => (
                  <li key={w}>{w}</li>
                ))}
              </ul>
            </>
          ) : (
            ", which is any moment now. "
          )}
          <Copyable text="ws daemon restart --now" className="mono">
            ws daemon restart --now
          </Copyable>{" "}
          doesn't wait.
        </Callout>
      )}
      {standing === "out" && (
        <Callout
          tone="attention"
          icon="arrow-up"
          title={`ws ${up.latest} is out`}
          actions={
            <Button variant="primary" onClick={() => void upgrade()} disabled={!!busy}>
              <Icon name={busy === "upgrade" ? "spinner" : "arrow-up"} size={12} /> {busy === "upgrade" ? "Upgrading…" : `Upgrade to ${up.latest}`}
            </Button>
          }
        >
          This is {shortVersion(up.version)}. {notes}
        </Callout>
      )}
      {standing === "out-built" && (
        <Callout tone="attention" icon="arrow-up" title={`ws ${up.latest} is out`}>
          This is {shortVersion(up.version)}, built from a clone rather than installed from a release, so it's changed where it was built: pull, and{" "}
          <span className="mono">just build</span>. {notes}
        </Callout>
      )}
      <dl className="settings-facts">
        <dt>Latest release</dt>
        <dd>
          {up.latest ? (
            <>
              <span className="mono">{up.latest}</span>
              {standing === "latest" && <span className="dim"> · this is it</span>}
              {standing === "ahead" && <span className="dim"> · this ws is later</span>}
              {(standing === "latest" || standing === "ahead") && notes && <> · {notes}</>}
            </>
          ) : (
            <span className="dim">{up.error ? "Not known" : up.tried ? "None yet" : "Not looked for yet"}</span>
          )}
          {up.error && (
            <div className="settings-update-error">
              <Icon name="alert" size={12} /> The last look didn't get an answer: {humanError(up.error)}
            </div>
          )}
        </dd>
        <dt>Last looked</dt>
        <dd className="settings-update-looked">
          <span>
            {up.checked ? (
              <Time iso={up.checked} as="relative" />
            ) : up.tried ? (
              <span className="dim">
                Tried <Time iso={up.tried} as="relative" />, with no answer
              </span>
            ) : (
              <span className="dim">Never</span>
            )}
            <span className="dim"> · {up.checks ? "the daemon looks once a day" : "the daemon doesn't look by itself"}</span>
          </span>
          <Button size="small" onClick={look} disabled={!!busy} title="Ask now for the latest release of ws, and of each plugin installed from a repo">
            <Icon name={busy === "look" ? "spinner" : "rotate"} size={12} /> {busy === "look" ? "Looking…" : "Check now"}
          </Button>
        </dd>
        {up.plugins > 0 && (
          <>
            <dt>Plugins</dt>
            <dd>
              <Link to="/settings/plugins" className="link">
                {up.plugins} {up.plugins === 1 ? "plugin has" : "plugins have"} a later release
              </Link>
            </dd>
          </>
        )}
      </dl>
    </div>
  );
}
