import { useLayoutEffect, useRef, useState } from "react";
import { useKeys } from "../keys";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import css from "./CodeNavCard.module.css";
import { CodeNavList, codeNavListWhereClass } from "./CodeNavList";
import { CodeNavSig } from "./CodeNavSig";
import { answerWords, type NavAnswer, type NavTarget } from "./nav";
import { Text } from "../ui/Text";

// CodeNavCard is a name's card: where it's defined and what it says, or
// the definitions it may be, to pick from (j, k, ⏎). It's placed under
// the name (rect), or over it where there isn't room. One opened by a
// click or d (pinned) stays until it's closed (esc), and its keys are its
// own while it's open. definedHere is a name at its definition: the card
// goes to who uses it.
export function CodeNavCard({
  answer: a,
  rect,
  pinned,
  definedHere,
  onClose,
  onGo,
  onFollow,
}: {
  answer?: NavAnswer; // undefined while it's asked
  rect: DOMRect;
  pinned: boolean;
  definedHere?: boolean;
  onClose: () => void;
  onGo: (t: NavTarget) => void;
  onFollow: () => void;
}) {
  const list = a && (a.answer === "candidates" || a.targets.length > 1) ? a.targets : undefined;
  const [pick, setPick] = useState(0);
  useKeys(
    pinned
      ? {
          Escape: onClose,
          j: () => setPick((i) => Math.min(i + 1, (list?.length ?? 1) - 1)),
          ArrowDown: () => setPick((i) => Math.min(i + 1, (list?.length ?? 1) - 1)),
          k: () => setPick((i) => Math.max(i - 1, 0)),
          ArrowUp: () => setPick((i) => Math.max(i - 1, 0)),
          Enter: () => (list ? onGo(list[pick]) : onFollow()),
        }
      : {},
    { modal: pinned },
  );
  const ref = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number }>();
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const h = el.offsetHeight;
    const w = el.offsetWidth;
    const below = rect.bottom + 6;
    const top = below + h > window.innerHeight - 8 ? Math.max(8, rect.top - h - 6) : below;
    setPos({ top, left: Math.max(8, Math.min(rect.left, window.innerWidth - w - 8)) });
  }, [rect, a]);
  const one = a && !list ? a.targets[0] : undefined;
  return (
    <div ref={ref} className={cx(css.card, pinned && css.pinned)} style={pos ?? { visibility: "hidden" }} onMouseDown={(e) => e.stopPropagation()}>
      {!a && <Text as="div" dim>Looking…</Text>}
      {one && (
        <button className={css.target} onClick={() => onGo(one)}>
          {one.signature && <CodeNavSig>{one.signature}</CodeNavSig>}
          {one.doc && <div className={css.doc}>{one.doc}</div>}
          <Text as="div" mono dim className={css.where}>
            {one.path}:{one.line}
          </Text>
        </button>
      )}
      {a && !one && !list && <CodeNavSig as="div">{a.external?.module ? `${a.external.module}${a.external.name ? `.${a.external.name}` : ""}` : a.name}</CodeNavSig>}
      {list && (
        <CodeNavList
          rows={list.map((t, i) => ({
            key: `${t.path}:${t.line}`,
            picked: i === pick,
            onEnter: () => setPick(i),
            onClick: () => onGo(t),
            children: (
              <>
                <CodeNavSig>{t.signature || `${t.kind} ${a!.name}`}</CodeNavSig>
                <Text mono dim className={codeNavListWhereClass}>
                  {t.path}:{t.line}
                </Text>
              </>
            ),
          }))}
        />
      )}
      {a && (
        <Text as="div" dim className={css.how}>
          <span className={a.answer === "resolved" ? css.resolved : a.answer === "candidates" || a.answer === "unresolved" ? css.unsure : undefined}>{definedHere ? "Defined here: click for uses" : answerWords(a)}</span>
          {a.answer === "candidates" && a.how && <span> · {a.how}</span>}
          {a.partial && <span title="Search incomplete: may have missed some"> · partial</span>}
          {a.external?.url && (
            <a href={a.external.url} target="_blank" rel="noreferrer">
              Docs <Icon name="external" size={10} />
            </a>
          )}
          {pinned && <Keys k="esc" />}
        </Text>
      )}
    </div>
  );
}
