import { PinButton } from "../PinButton";
import { RefsScope } from "../../refs/known";
import { RefCardBody } from "../../refs/RefCardBody";
import { ThingCard } from "../../refs/ThingCard";
import { RefFoot } from "../../refs/RefFoot";
import { type Known, namedOf, type PageRef, refAt, refKey } from "../../refs/refs";
import { RefTag } from "../../refs/RefTag";
import { HoverCardLine } from "../../ui/HoverCard";
import { Icon } from "../../ui/Icon";
import { Status } from "../../ui/Status";
import { capital } from "../../ui/text";
import { actIcon, type ActOn, actsOn, type Found, type Item } from "../find";
import { FindAct } from "./FindAct";
import { FindActs } from "./FindActs";
import { FindCodeCard } from "./FindCodeCard";
import { FindItemCard } from "./FindItemCard";
import { FindPane } from "./FindPane";
import { FindPullCard } from "./FindPullCard";
import { FindSaidCard } from "./FindSaidCard";
import { FindThing } from "./FindThing";

// Thing is a plugin's thing, as a ref.
type Thing = Extract<PageRef, { kind: "thing" }>;

// PalettePane is the pane beside Find's list, saying what the cursor's on
// (on): a line of a file, a pull request, a plugin's thing (thing: the
// one the cursor's on, or the one Find's acting on), something found by
// what it says, anything else with a card, and failing those what its
// row says. finding: Find's finding, not in the middle of doing
// something to a thing, when its pane says less.
export function PalettePane({
  on,
  thing,
  finding,
  known,
  working,
  onDo,
  onClose,
}: {
  on?: Item;
  thing?: Thing;
  finding: boolean;
  known: Known;
  working: boolean; // something's being done: nothing else is started
  onDo: (thing: Thing, act: ActOn) => void;
  onClose: () => void;
}) {
  // A name one of a thing's fields holds: as what it names, where ws
  // reads it as a name, and its words where it doesn't.
  const pointer = (name: string) => {
    const ref = refAt(name, known);
    return ref ? (
      <RefTag refKey={refKey(ref)} inline>
        {name}
      </RefTag>
    ) : (
      name
    );
  };
  const said = (it: Item, f: Found) => <FindSaidCard icon={it.icon} title={it.title} about={saidAbout(it.kind, f)} snippet={f.snippet} />;
  return (
    <FindPane scrolls={!on?.code && !on?.pull && !!thing}>
      {on?.code ? (
        <FindCodeCard path={on.code.path} line={on.code.line} repo={on.code.repo} worktree={on.code.worktree} />
      ) : on?.pull ? (
        <FindPullCard pull={on.pull} />
      ) : thing ? (
        // A plugin's thing: its fields, as its plugin has it drawn here.
        // What the search found of it shows until they're in hand, and if
        // they can't be had. A name one of its fields holds is what it
        // names, to go to (which closes Find), and under it is what can
        // be done to it.
        <RefsScope known={known}>
          <FindThing onGo={onClose}>
            <ThingCard r={thing} known={known} surface="pane" pointer={pointer} before={finding && on?.said ? said(on, on.said) : undefined} />
          </FindThing>
          <FindActs>
            {finding && <PinButton r={thing} keys="mod+." />}
            {actsOn(namedOf(thing, known), thing.id).map((a) => (
              <FindAct key={`${a.ws ?? "its"}:${a.id}`} icon={actIcon(a.icon)} disabled={working} onClick={() => onDo(thing, a)}>
                {a.title}
              </FindAct>
            ))}
          </FindActs>
        </RefsScope>
      ) : on?.said ? (
        said(on, on.said)
      ) : on?.card ? (
        <>
          <RefCardBody r={on.card} known={known} pane />
          <RefFoot>
            <PinButton r={on.card} end keys={finding ? "mod+." : undefined} />
          </RefFoot>
        </>
      ) : on ? (
        <FindItemCard mark={on.status ? <Status s={on.status} iconOnly size={13} /> : <Icon name={on.icon ?? "chevron-right"} size={13} className="dim" />} title={on.title} kind={on.kind} hint={on.hint} />
      ) : (
        <HoverCardLine dim>What the cursor's on shows here.</HoverCardLine>
      )}
    </FindPane>
  );
}

// saidAbout is what's known of something found by what it says: what it
// is (the row's kind, where the search doesn't say), where, what it's on,
// its state, and that it's archived.
export function saidAbout(kind: string, f: Found): string[] {
  return [f.what ? capital(f.what) : kind, f.where, f.on ? `on ${f.on}` : "", f.state && f.place !== "brief" ? f.state : "", f.ended ? "archived" : ""].filter((s): s is string => !!s);
}
