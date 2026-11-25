import { Icon } from "../../ui/Icon";
import { Inline } from "../../ui/Inline";
import css from "./MissionEndingZ.module.css";
import { Text } from "../../ui/Text";

// A part of Z as it was checked: met, not met, or nobody checked it; and
// how, in the checker's words.
export type EndingZPart = { z: string; how?: string; stands: "met" | "unmet" | "unchecked" };

const standing = { met: css.met, unmet: css.unmet, unchecked: css.unchecked };

// MissionEndingZ is Z as it was checked, a part a line: a tick, a cross or
// a dashed ring, the part, and how it was checked (or that it wasn't).
export function MissionEndingZ({ parts }: { parts: EndingZPart[] }) {
  return (
    <ol className={css.list}>
      {parts.map((z, i) => (
        <li key={i} className={standing[z.stands]}>
          <Icon name={z.stands === "met" ? "check-circle" : z.stands === "unmet" ? "x-circle" : "circle-dashed"} size={13} />
          <span>
            <span>
              <Inline text={z.z} />
            </span>
            {z.how && (
              <span className={css.how}>
                {" "}
                — <Inline text={z.how} />
              </span>
            )}
            {z.stands === "unchecked" && <Text dim className={css.how}> — not checked</Text>}
          </span>
        </li>
      ))}
    </ol>
  );
}
