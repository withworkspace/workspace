import { useState } from "react";
import { resyncSilo, type SiloState } from "../api";
import type { Reload } from "../load";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Printed } from "../ui/Printed";
import { Spinner } from "../ui/Spinner";
import { humanError } from "../ui/strings";
import { Time } from "../ui/Time";
import { toast } from "../ui/toast";
import { RunMark } from "./RunMark";
import { RunsPageMore, runsPageLogClass } from "./RunsPageMore";
import { RunsPageRow } from "./RunsPageRow";
import { RunsPageSub } from "./RunsPageSub";
import { Text } from "../ui/Text";

// RunsSiloRow is one of the repos' silos, as the daemon keeping it has
// it: what it's pointed at, its last sync, and its last hook, opened
// under it; and Resync.
export function RunsSiloRow({ st, reload }: { st: SiloState; reload: Reload }) {
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState(false);
  const last = st.syncs?.[st.syncs.length - 1];
  const hook = st.hook;
  const trouble = st.error || last?.error || hook?.error;
  const resync = () => {
    setBusy(true);
    resyncSilo(st.repo)
      .then(() => {
        toast(`Resynced ${st.repo}'s silo`, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  return (
    <RunsPageRow
      open={open}
      mark={<RunMark live={st.watching} up={st.watching} title={st.watching ? "Syncing as files change" : "Not syncing"} />}
      name={st.repo}
      top={
        <>
          <Text dim>on</Text>
          <Text mono>{st.target}</Text>
          {trouble && <Chip tone="waiting">{st.error ? "not syncing" : last?.error ? "sync failed" : `${hook?.name} failed`}</Chip>}
        </>
      }
      sub={
        <RunsPageSub>
          {st.error ? (
            <span>{st.error}</span>
          ) : (
            <span>
              Watching {st.dirs} folder{st.dirs === 1 ? "" : "s"}
            </span>
          )}
          {last && (
            <span>
              {last.error ? "Last sync failed" : `Last synced ${last.files} file${last.files === 1 ? "" : "s"}`}
              {last.full && last.why ? ` (${last.why})` : ""}, <Time iso={last.at} as="relative" />
            </span>
          )}
          {hook && (
            <span>
              {hook.name} {hook.error ? "failed" : "ran"} <Time iso={hook.at} as="relative" />
            </span>
          )}
        </RunsPageSub>
      }
      actions={
        <>
          {(hook || last?.error) && (
            <Button variant="ghost" size="small" onClick={() => setOpen(!open)} aria-expanded={open} title={open ? "Hide" : "Last hook output and sync errors"}>
              <Icon name={open ? "chevron-up" : "terminal"} size={11} /> {open ? "Hide" : "Output"}
            </Button>
          )}
          <Button variant="ghost" size="small" disabled={busy || !!st.error} onClick={resync} title="Full resync from capsule">
            {busy ? <Spinner size={11} /> : <Icon name="rotate" size={11} />} Resync
          </Button>
        </>
      }
      more={
        <RunsPageMore>
          {last?.error && <p>{last.error}</p>}
          {hook && (
            <>
              <Text as="p" dim>
                {hook.name} hook{hook.error ? `: ${hook.error}` : ""}
              </Text>
              <Printed className={runsPageLogClass}>{hook.output || "No output."}</Printed>
            </>
          )}
        </RunsPageMore>
      }
    />
  );
}
