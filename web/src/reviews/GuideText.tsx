import type { GuideText as Guide } from "../api";
import { Inline } from "../ui/Inline";
import css from "./GuideText.module.css";
import { Text } from "../ui/Text";

// GuideText is a guide: what the change does, where that differs from its
// description, and its groups. One being written (writing) is as far as
// it's got: nothing's said of what differs until it is, and it ends on a
// caret.
export function GuideText({ g, writing }: { g: Guide; writing?: boolean }) {
  const caret = writing ? <span className={css.caret} aria-hidden /> : null;
  const groups = g.groups.length > 0;
  return (
    <div className={writing ? css.draft : undefined} aria-busy={writing || undefined}>
      {g.does && (
        <p className={css.does}>
          <Inline text={g.does} />
          {!g.differs?.length && !groups && caret}
        </p>
      )}
      {g.differs?.length ? (
        <div className={css.differs}>
          <strong>Where it differs from what it says it does</strong>
          <ul>
            {g.differs.map((d, i) => (
              <li key={i}>
                <Inline text={d} />
                {i === g.differs!.length - 1 && !groups && caret}
              </li>
            ))}
          </ul>
        </div>
      ) : (
        // Being written, it's said nothing differs once it's on to the files.
        (!writing || groups) && <Text as="p" dim className={css.same}>It does what its description says.</Text>
      )}
      {groups && (
        <ol className={css.groups}>
          {g.groups.map((x, i) => (
            <li key={i} className={x.skim ? css.skim : ""}>
              <strong>{x.name}</strong>{" "}
              <Text dim>
                · {x.files.length} {x.files.length === 1 ? "file" : "files"}
                {writing && i === g.groups.length - 1 ? " so far" : ""}
                {x.skim ? " · to skim" : ""}
              </Text>
              {x.why && (
                <span className={css.why}>
                  <Inline text={x.why} />
                  {i === g.groups.length - 1 && caret}
                </span>
              )}
              {!x.why && i === g.groups.length - 1 && caret}
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
