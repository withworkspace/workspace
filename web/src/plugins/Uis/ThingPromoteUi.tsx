import { useEffect, useRef, useState } from "react";
import * as api from "../../api";
import { OverFrame } from "./OverFrame";
import { ThingPromote } from "./ThingPromote";
import { asked, grouped, said } from "./things";
import type { UiProps } from "./uiProps";

// ws's UIs for a plugin's thing that lives in a repo, asked for by the
// plugin's page from a button of its own (ui.ts): putting one there
// (thing-promote, here) and saving it (thing-save, ThingSaveUi.tsx).
// Where a thing goes, the commit and the landing are ws's, so what says
// them is ws's too, the same for every plugin. ws asks the plugin for the
// thing itself, and tells it where it is: the page is given what was
// done, and nothing passes through it.

// ThingPromoteUi puts one of a plugin's own things in a repo: it says
// where it would live, or, where its kind's things go in more places than
// one, has you pick among them; takes a folder of the place to group it
// in, where its kind's things may be grouped, with those in use to pick
// from; takes somewhere else if you say; and does it when you say. What
// comes of it is where the thing is now.
export function ThingPromoteUi({ asked: asking, plugin, anchor, done }: UiProps) {
  const thing = asked(asking.with);
  const [places, setPlaces] = useState<api.ThingPlace[]>();
  const [failed, setFailed] = useState("");
  // Which of the places, or somewhere else, said in words.
  const [pick, setPick] = useState<number | "else">(0);
  const [elsewhere, setElsewhere] = useState("");
  // A folder of the place picked, to group it in: for a place that takes one.
  const [under, setUnder] = useState("");
  const [busy, setBusy] = useState(false);
  const go = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    let live = true;
    api.thingOffered(plugin, thing.kind, thing.title ?? "").then(
      (o) => {
        if (!live) return;
        setPlaces(o.places);
        // Nowhere to offer: it's said in words.
        if (o.places.length === 0) setPick("else");
      },
      (err) => {
        if (!live) return;
        setFailed(said(err));
        setPlaces([]);
        setPick("else");
      },
    );
    return () => {
      live = false;
    };
  }, [plugin, thing.kind, thing.title]);
  useEffect(() => go.current?.focus(), [places]);

  // Another place has folders of its own: the one said for the last isn't carried to it.
  const choose = (next: number | "else") => {
    if (next !== pick) setUnder("");
    setPick(next);
  };
  const place = pick === "else" ? undefined : places?.[pick];
  const group = place?.subfolders ? under.trim().replace(/^\/+|\/+$/g, "") : "";
  const to = pick === "else" ? elsewhere.trim() : place?.where;
  const promote = async () => {
    if (!to) return;
    setBusy(true);
    setFailed("");
    try {
      // A place is named, not its folder: ws names the thing's own afresh, as the repo is on its remote then. The first is where one goes with nothing said.
      const where = pick === "else" ? { to } : { ...(pick === 0 ? {} : { place: place?.place }), ...(group ? { under: group } : {}) };
      done(await api.promoteThing(plugin, { kind: thing.kind, ref: thing.ref, ...where }));
    } catch (err) {
      setFailed(said(err));
      setBusy(false);
    }
  };
  const what = thing.title || thing.ref;
  return (
    <OverFrame anchor={anchor} label={`Put ${what} in a repo`} busy={busy} dismiss={() => done()}>
      <ThingPromote
        what={what}
        places={places?.map((p, i) => ({
          key: p.where,
          label: p.label,
          where: pick === i ? grouped(p.where, group) : p.where,
          direct: p.direct,
          folders: p.subfolders ? (p.groups ?? []) : undefined,
        }))}
        pick={pick}
        onPick={choose}
        under={under}
        group={group}
        onUnder={setUnder}
        elsewhere={elsewhere}
        onElsewhere={setElsewhere}
        busy={busy}
        failed={failed}
        ready={!!to}
        go={go}
        onPromote={() => void promote()}
        onCancel={() => done()}
      />
    </OverFrame>
  );
}
