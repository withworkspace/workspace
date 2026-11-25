import { Chip } from "../ui/Chip";
import css from "./TalkProposalMeta.module.css";

// TalkProposalMeta is what else a card says of what's proposed (what it
// lifts, where it's added, what it's committed as), a chip each; nothing,
// when there's none.
export function TalkProposalMeta({ items }: { items: (string | false | undefined)[] }) {
  const list = items.filter(Boolean) as string[];
  if (!list.length) return null;
  return (
    <div className={css.meta}>
      {list.map((m) => (
        <Chip tone="quiet" key={m}>
          {m}
        </Chip>
      ))}
    </div>
  );
}
