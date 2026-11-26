import { useRefs } from "../../refs/known";
import { refAt, refKey } from "../../refs/refs";
import { RefTag } from "../../refs/RefTag";
import { Text } from "../../ui/Text";

// SideAbout is one thing a mission's about, by its name: drawn as what it
// names (a plugin's thing, a pull request, anything ws names), with its
// card, and as its words where ws doesn't read it as a name.
export function SideAbout({ name }: { name: string }) {
  const known = useRefs()?.known;
  const ref = known ? refAt(name, known) : undefined;
  return ref ? <RefTag refKey={refKey(ref)}>{name}</RefTag> : <Text mono>{name}</Text>;
}
