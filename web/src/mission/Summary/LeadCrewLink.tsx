import { Link } from "react-router";
import { buttonClass } from "../../ui/Button";
import { Icon } from "../../ui/Icon";

// LeadCrewLink is the way to crew's page from a card that's about them,
// when they're not stopped on anything to answer here.
export function LeadCrewLink({ slug, id, verb }: { slug: string; id: string; verb: string }) {
  return (
    <Link to={`/missions/${slug}/crew/${id}`} className={buttonClass({ variant: "primary" })}>
      <Icon name="message" /> {verb === "watch" ? "Watch" : "Open"} {id}
    </Link>
  );
}
