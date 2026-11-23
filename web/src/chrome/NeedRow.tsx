import { Status } from "../ui/status";
import { type Need, needSays, needStatus } from "./need";
import { SidebarRow } from "./SidebarRow";

type NeedRowProps = {
  need: Need;
  // The page you're on is where it's dealt with.
  on?: boolean;
  // It has just arrived: it lights up once, where it is. That's all a
  // new need gets: it's no notification, and nothing covers the page.
  fresh?: boolean;
};

// NeedRow is one thing that needs you, in Needs you: how it stands and
// what it is, a press from where it's dealt with.
export function NeedRow({ need, on, fresh }: NeedRowProps) {
  return (
    <SidebarRow
      to={need.to}
      lead={<Status s={needStatus(need)} iconOnly />}
      title={need.title}
      says={needSays(need) || undefined}
      on={on}
      fresh={fresh}
    />
  );
}
