import { Fragment, useEffect, useState } from "react";
import { getPR, type PR } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Text } from "../ui/Text";
import { Prose } from "../ui/Prose";

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
              {i + 1}. {l.title} <Text mono>#{l.number}</Text>
            </h2>
            {body === undefined ? (
              <Text as="p" dim>Reading it…</Text>
            ) : body === null ? (
              <Text as="p" dim>Its description couldn't be read.</Text>
            ) : body.trim() ? (
              <Prose>
                <Markdown text={body} />
              </Prose>
            ) : (
              <Text as="p" dim>It has no description.</Text>
            )}
          </Fragment>
        );
      })}
    </>
  );
}
