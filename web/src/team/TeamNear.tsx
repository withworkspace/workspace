import type { TeamItem } from "../api";
import { TeamKnowLine } from "./TeamKnowLine";
import { Text } from "../ui/Text";

type Near = NonNullable<TeamItem["near"]>[number];

// nearSaid is how many capsules are each said on a line of their own: a
// change near more of them than that says so on one line between them.
const nearSaid = 3;

const kind = { label: "Near you", icon: "box", tone: "progress", title: "A capsule of yours has changed some of the same files" } as const;

// TeamNear is where a change is near you, under its row: the capsules of
// yours that have changed some of the files it changes. A few are a line
// each, with the files; many are one line, however many they are, so a
// row is never a page of them.
export function TeamNear({ near }: { near: Near[] }) {
  if (near.length > nearSaid) {
    return (
      <TeamKnowLine kind={kind} title={near.map((n) => n.capsule).join("\n")}>
        Also changed in {near.length} of your capsules: <Text mono>{near[0].capsule}</Text>, <Text mono>{near[1].capsule}</Text> and {near.length - 2} more
      </TeamKnowLine>
    );
  }
  return near.map((n) => (
    <TeamKnowLine key={n.capsule} kind={kind} title={n.files.join("\n")}>
      Also changed in your <Text mono>{n.capsule}</Text>: {n.files.slice(0, 2).map((f) => f.split("/").pop()).join(", ")}
      {n.files.length > 2 ? `, and ${n.files.length - 2} more` : ""}
    </TeamKnowLine>
  ));
}
