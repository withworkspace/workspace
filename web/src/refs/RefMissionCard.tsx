import type { Mission } from "../api";
import { Inline } from "../ui/Inline";
import { Status } from "../ui/Status";
import { headlineStatus } from "../ui/statusOf";
import { type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefGo } from "./RefGo";
import { Text } from "../ui/Text";

// RefMissionCard is a mission's card: its title, and its headline.
export function RefMissionCard({ r, mission: m }: { r: Extract<PageRef, { kind: "mission" }>; mission?: Mission }) {
  return (
    <>
      <RefCardHead icon="flag" title={m?.title ?? r.slug} />
      {m && (
        <>
          <Status s={headlineStatus(m.headline)} />
          {m.headline.text && (
            <Text dim>
              <Inline text={m.headline.text} lines={3} />
            </Text>
          )}
        </>
      )}
      <RefGo to={refHref(r)}>Open mission</RefGo>
    </>
  );
}
