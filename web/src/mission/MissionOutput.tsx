import type { Artifact } from "../api";
import { Chip } from "../ui/Chip";
import { Copyable } from "../ui/Copyable";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { Time } from "../ui/Time";
import css from "./MissionOutput.module.css";
import { MissionOutputState } from "./MissionOutputState";
import { MissionOutputThreads } from "./MissionOutputThreads";
import { artifactTitle, isArtifact } from "./outputs";
import { textClass } from "../ui/Text";

const kindIcon: Record<string, IconName> = {
  proposal: "file",
  adr: "gavel",
  note: "book",
  doc: "file",
  handoff: "arrow-right",
  review: "eye",
  plan: "layers",
  artifact: "artifact",
};

// MissionOutput is one of a mission's outputs (a document, a pull request,
// an artifact) as a row: its kind's icon, a short title, what the agent
// said of it under, and its marks. compact is the sidebar's, a line of
// each with its kind and state under. One that can be read opens on a
// click or ⏎.
export function MissionOutput({ a, selected, open, compact, id }: { a: Artifact; selected?: boolean; open: (a: Artifact) => void; compact?: boolean; id?: string }) {
  const { title, desc } = artifactTitle(a);
  const openable = a.readable || !!a.url || isArtifact(a);
  return (
    <div
      id={id}
      className={cx(css.row, selected && css.selected, openable && css.openable, compact && css.compact)}
      onClick={() => openable && open(a)}
      onKeyDown={(e) => e.key === "Enter" && openable && open(a)}
      role={openable ? "link" : undefined}
      tabIndex={openable ? 0 : undefined}
      title={desc}
    >
      <Icon name={kindIcon[a.kind] ?? "file"} size={14} className={css.icon} />
      <div className={css.body}>
        <div className={css.line}>
          <span className={css.title}>{title}</span>
          {isArtifact(a) && !!a.versions && (
            <Chip tone="quiet" title={`${a.versions} ${a.versions === 1 ? "version" : "versions"}`}>
              v{a.versions}
            </Chip>
          )}
          {(!compact || isArtifact(a)) && <MissionOutputState state={a.state} />}
          {a.threads && <MissionOutputThreads t={a.threads} />}
          {!a.readable && a.url && <Icon name="external" size={12} className={textClass("dim")} />}
        </div>
        {desc && (
          <div className={css.desc}>
            <Inline text={desc} lines={compact ? 1 : 2} />
          </div>
        )}
        {compact ? (
          <div className={css.meta}>{isArtifact(a) ? `${a.ref}${a.by ? ` · ${a.by}` : ""}` : `${a.kind} · ${a.state}`}</div>
        ) : (
          <div className={css.meta}>
            {a.by && <span>{a.by}</span>}
            {a.added && <Time iso={a.added} as="relative" />}
            <Copyable text={a.ref} title="Copy its ref" />
          </div>
        )}
      </div>
    </div>
  );
}
