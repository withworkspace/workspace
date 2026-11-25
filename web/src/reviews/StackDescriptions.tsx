import { Fragment, useEffect, useState } from "react";
import { getPR, type PR } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";

// StackDescriptions is a whole stack's overview: each layer's
// description, bottom first, which is the order to read them in.
export function StackDescriptions({ layers }: { layers: PR[] }) {
  const [bodies, setBodies] = useState<Record<number, string | null>>({});
  const key = layers.map((l) => `${l.repo}#${l.number}`).join(" ");
  useEffect(() => {
    let stopped = false;
    for (const l of layers) {
      getPR(l.repo, l.number).then(
        (d) => !stopped && setBodies((b) => ({ ...b, [l.number]: d.body ?? "" })),
        () => !stopped && setBodies((b) => ({ ...b, [l.number]: null })),
      );
    }
    return () => {
      stopped = true;
    };
  }, [key]);
  return (
    <>
      {layers.map((l, i) => {
        const body = bodies[l.number];
        return (
          <Fragment key={l.number}>
            <h2>
              {i + 1}. {l.title} <span className="mono">#{l.number}</span>
            </h2>
            {body === undefined ? (
              <p className="dim">Reading it…</p>
            ) : body === null ? (
              <p className="dim">Its description couldn't be read.</p>
            ) : body.trim() ? (
              <div className="prose">
                <Markdown text={body} />
              </div>
            ) : (
              <p className="dim">It has no description.</p>
            )}
          </Fragment>
        );
      })}
    </>
  );
}
