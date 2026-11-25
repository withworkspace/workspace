import type { ReactNode } from "react";
import type { FocusPin, Workspace } from "../api";
import { useLoaded } from "../load";
import { prOf } from "../refs/prs";
import { type Known, namedOf, type PageRef } from "../refs/refs";
import { thingIcon } from "../refs/views";
import type { IconName } from "../ui/Icon";
import { Status } from "../ui/Status";
import { StatusIcon } from "../ui/StatusIcon";
import { headlineStatus } from "../ui/statusOf";

const quiet = (name: IconName) => <StatusIcon icon={name} tone="quiet" />;

// useStands is how a pin stands, as an icon, and what it's called: from
// what the workspace says of it where it says (a mission's headline, a
// conversation waiting on you), else what the graph said.
export function useStands(pin: FocusPin, r: PageRef, workspace: Workspace, known?: Known): { icon: ReactNode; title: string } {
  const pr = useLoaded(r.kind === "pr" ? () => prOf(r.repo, r.number) : null, [r.kind === "pr" ? `${r.repo}#${r.number}` : ""]).value;
  switch (r.kind) {
    case "mission": {
      const m = workspace.missions.find((x) => x.slug === r.slug);
      return {
        icon: m ? <Status s={headlineStatus(m.headline)} iconOnly /> : quiet("flag"),
        title: m?.title ?? pin.title ?? r.slug,
      };
    }
    case "talk": {
      const t = workspace.conversations.find((x) => x.id === r.id);
      const waits = !!t?.waiting && t.status === "open";
      return {
        icon: waits ? <StatusIcon icon="reply" tone="attention" title="Waiting on you" /> : quiet("message"),
        title: t?.title || t?.opening || pin.title || r.id,
      };
    }
    case "pr": {
      const tone = pr?.checks === "failure" ? "danger" : pr?.state === "needs-review" || pr?.state === "back-with-you" ? "attention" : "quiet";
      return {
        icon: <StatusIcon icon="pr" tone={tone} title={pr?.checks === "failure" ? "Its checks fail" : undefined} />,
        title: `${r.repo}#${r.number} ${pr?.title ?? pin.title ?? ""}`.trim(),
      };
    }
    case "thing":
      return {
        icon: quiet(thingIcon(namedOf(r, known))),
        title: `${r.id} ${pin.title ?? ""}`.trim(),
      };
    case "page":
      return {
        icon: quiet("file"),
        title: workspace.pages?.find((p) => p.id === r.id)?.title || pin.title || r.id,
      };
    case "artifact":
      return {
        icon: quiet("artifact"),
        title: workspace.artifacts?.find((a) => a.id === r.id)?.title || pin.title || r.id,
      };
    case "dir":
      return { icon: quiet("layers"), title: pin.title || `${r.path}/` };
    case "file":
      return {
        icon: quiet("file"),
        title: pin.title || r.path.split("/").pop() || r.path,
      };
  }
  return { icon: quiet("circle"), title: pin.title || pin.name };
}
