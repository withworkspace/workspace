import { useState } from "react";
import { type InstalledPlugin, removePlugin, updatePlugin } from "../../api";
import type { Reload } from "../../load";
import { Button } from "../../ui/Button";
import { Copyable } from "../../ui/Copyable";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { humanError } from "../../ui/strings";
import { toast } from "../../ui/toast";
import { addsOf, changedSaid, fromOf, repoLink, stateOf, updateOf } from "../installed";
import { pluginIcon } from "../pluginIcon";
import { type Failed, failed } from "./failed";
import { PluginsRemove } from "./PluginsRemove";
import { PluginsRow, PluginsRowNote } from "./PluginsRow";
import { Text } from "../../ui/Text";

// InstalledRow is an installed plugin, with updating and removing it.
// icon is its first view's, as the sidebar draws it.
export function InstalledRow({ p, icon, reload, onFailed }: { p: InstalledPlugin; icon?: string; reload: Reload; onFailed: (f?: Failed) => void }) {
  const [busy, setBusy] = useState<"update" | "remove">();
  const [removing, setRemoving] = useState(false);
  const state = stateOf(p);
  const update = updateOf(p);
  const doUpdate = () => {
    setBusy("update");
    onFailed(undefined);
    updatePlugin(p.name)
      .then(async (c) => {
        await reload();
        toast(changedSaid(c, "updated"), { tone: "done", ms: 6000 });
      })
      .catch(async (e: Error) => {
        await reload();
        onFailed(failed(`Could not update ${p.name}`, e));
      })
      .finally(() => setBusy(undefined));
  };
  const doRemove = async (data: boolean) => {
    setRemoving(false);
    setBusy("remove");
    onFailed(undefined);
    try {
      await removePlugin(p.name, data);
      await reload();
      toast(`Removed ${p.name}${data ? ", and what it kept" : ""}`, { tone: "done" });
    } catch (e) {
      await reload();
      onFailed(failed(`Could not remove ${p.name}`, e));
    } finally {
      setBusy(undefined);
    }
  };
  return (
    <PluginsRow
      kind={p.state === "ready" || p.state === "failed" ? p.state : undefined}
      icon={p.state === "failed" ? "alert" : pluginIcon(icon)}
      // By its name, not its view's title: a view may be called anything
      // (plugin-dev's is "Plugins"), and what it adds is said under it.
      name={p.name}
      to={p.path && p.state !== "failed" ? p.path : undefined}
      version={p.version}
      state={state}
      out={update.out ? { label: `${p.latest} available`, href: repoLink(p.from.repo ?? "", p.latest) } : undefined}
      about={p.description}
      from={{ label: fromOf(p), href: p.from.repo ? repoLink(p.from.repo) : undefined, at: p.from.at }}
      adds={addsOf(p)}
      notes={
        <>
          {p.state === "setup" && (
            <PluginsRowNote tone="attention">
              Not usable until {p.missing?.length === 1 ? "this is" : "these are"} set:{" "}
              <Text mono>{(p.missing ?? []).join(", ")}</Text>. In a terminal:{" "}
              <Copyable text={`ws plugin config ${p.name} --ask`}>ws plugin config {p.name} --ask</Copyable>
            </PluginsRowNote>
          )}
          {p.state === "failed" && (
            <PluginsRowNote tone="danger">
              {humanError(p.error ?? "Could not start")} Logs:{" "}
              <Copyable text={`ws plugin logs ${p.name}`}>ws plugin logs {p.name}</Copyable>
            </PluginsRowNote>
          )}
          {p.state !== "failed" && p.error && <PluginsRowNote>Last log: {p.error}</PluginsRowNote>}
          {p.refused?.map((r) => (
            <PluginsRowNote key={r}>Left out: {r}</PluginsRowNote>
          ))}
          {p.updateError && <PluginsRowNote>Could not check for updates: {p.updateError}</PluginsRowNote>}
        </>
      }
      actions={
        <>
          {update.can && (
            <Button variant={update.out ? "primary" : "ghost"} size="small" onClick={doUpdate} disabled={!!busy} title={update.title}>
              {busy === "update" ? <Spinner size={11} /> : <Icon name={update.out ? "arrow-up" : "rotate"} size={11} />} {busy === "update" ? "Updating…" : update.label}
            </Button>
          )}
          <Button variant="ghost" size="small" onClick={() => setRemoving(true)} disabled={!!busy} title="Stop and remove from workspace">
            {busy === "remove" ? <Spinner size={11} /> : <Icon name="x" size={11} />} {busy === "remove" ? "Removing…" : "Remove"}
          </Button>
        </>
      }
    >
      {removing && <PluginsRemove name={p.name} names={p.names} onClose={() => setRemoving(false)} onRemove={doRemove} />}
    </PluginsRow>
  );
}
