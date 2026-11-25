import { useState } from "react";
import { headed, move, type Arranged } from "../places";
import { ArrangeList } from "./ArrangeList";
import { ArrangePanel } from "./ArrangePanel";

// Arrange is the sidebar while you arrange it: its places in their order,
// and under them those in the workspace's menu, each dragged by its grip
// (or picked up with Enter on it, from the keys) to another place in
// either list. Each also has a button that moves it across. Nothing is
// written until Done; Cancel leaves it as it was, Reset as the team's (or
// as it comes).
export function Arrange({
  start,
  canReset,
  onDone,
  onCancel,
  onReset,
}: {
  start: Arranged;
  canReset: boolean;
  onDone: (arranged: Arranged) => void;
  onCancel: () => void;
  onReset: () => void;
}) {
  const [arranged, setArranged] = useState(start);
  const put = (into: keyof Arranged) => (ids: string[], at?: { id: string; after: boolean }) => setArranged((a) => move(a, ids, into, at));
  const list = (it: keyof Arranged) => {
    const other: keyof Arranged = it === "sidebar" ? "menu" : "sidebar";
    return <ArrangeList list={it} places={arranged[it]} under={headed(arranged[it])} onPut={put(it)} onAcross={(id) => put(other)([id])} />;
  };
  return <ArrangePanel sidebar={list("sidebar")} menu={list("menu")} onReset={canReset ? onReset : undefined} onCancel={onCancel} onDone={() => onDone(arranged)} />;
}
