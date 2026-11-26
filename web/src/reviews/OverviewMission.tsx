import { Link } from "react-router";
import { Icon } from "../ui/Icon";
import css from "./OverviewMission.module.css";
import { Text, textClass } from "../ui/Text";

// OverviewMission is the mission a capsule's for, as a link, and the
// capsule: its repo, name and branch.
export function OverviewMission({ slug, title, repo, name, branch }: { slug: string; title: string; repo: string; name: string; branch?: string }) {
  return (
    <p className={css.mission}>
      <Link to={`/missions/${slug}`} className={textClass("link")}>
        <Icon name="flag" size={12} /> {title}
      </Link>{" "}
      <Text dim>
        · <Text mono>{repo}/{name}</Text>
        {branch && (
          <>
            {" "}
            on <Text mono>{branch}</Text>
          </>
        )}
      </Text>
    </p>
  );
}
