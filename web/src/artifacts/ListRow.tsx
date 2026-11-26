import type { ArtifactInfo } from "../api";
import { useThingMenu } from "../shell/ThingMenu";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { ArtifactRow, artifactRowCodeClass } from "./ArtifactRow";
import { From } from "./From";
import { artifactHref } from "./hrefs";
import { artifactStatus, behind, byWords, expiryOf, keptIcon, keptTitle, keptWords } from "./shown";
import { Text } from "../ui/Text";

// ListRow is an artifact of this machine's, in the list: where it stands,
// its latest version and its note, where it came from and where it's kept.
export function ListRow({ a, selected, onHover, onPromote }: { a: ArtifactInfo; selected: boolean; onHover: () => void; onPromote: () => void }) {
  const menu = useThingMenu();
  const v = a.latest;
  // A live one in its last week says when it expires, with Promote beside
  // it; an expired one is dimmed, until it's cleared.
  const expiry = a.status === "live" ? expiryOf(a, Date.now()) : undefined;
  const dimmed = a.status === "discarded" || !!expiry?.expired;
  return (
    <ArtifactRow
      to={artifactHref(a.id)}
      onContextMenu={menu(artifactHref(a.id), a.title)}
      id={`artifact-${a.id}`}
      option
      selected={selected}
      onHover={onHover}
      status={artifactStatus(a)}
      title={a.title}
      dimmed={dimmed}
      chips={
        <>
          <Chip tone="quiet" title={`${a.versions} ${a.versions === 1 ? "version" : "versions"}`}>
            v{v?.n ?? a.versions}
          </Chip>
          {a.status !== "live" || a.gone || a.expired ? <Chip tone={artifactStatus(a).tone}>{artifactStatus(a).words}</Chip> : null}
          {expiry?.soon && (
            <Chip tone="attention" title={`No new version since ${new Date(v?.at ?? a.made).toLocaleDateString()}. Promote it to keep it.`}>
              <Icon name="clock" size={11} /> {expiry.words}
            </Chip>
          )}
        </>
      }
      note={v?.note}
      when={v && { by: byWords(v), at: v.at, title: `v${v.n} ${byWords(v)}` }}
      onPromote={expiry?.soon || expiry?.expired ? onPromote : undefined}
    >
      {!!a.from?.length && (
        <span>
          from{" "}
          {a.from.map((f, i) => (
            <span key={f.from}>
              {i > 0 && ", "}
              <From from={f.from} title={f.title} inline />
            </span>
          ))}
        </span>
      )}
      <span title={keptTitle(a)}>
        <Icon name={keptIcon(a)} size={11} /> {keptWords(a)}
        {behind(a) > 0 && <Text dim>, v{a.latest?.n} here</Text>}
      </span>
      <Text mono className={artifactRowCodeClass}>{a.id}</Text>
    </ArtifactRow>
  );
}
