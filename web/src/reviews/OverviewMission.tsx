import { Link } from "react-router";
import { Icon } from "../ui/Icon";
import css from "./OverviewMission.module.css";

// OverviewMission is the mission a capsule's for, as a link, and the
// capsule: its repo, name and branch.
export function OverviewMission({ slug, title, repo, name, branch }: { slug: string; title: string; repo: string; name: string; branch?: string }) {
  return (
    <p className={css.mission}>
      <Link to={`/missions/${slug}`} className="link">
        <Icon name="flag" size={12} /> {title}
      </Link>{" "}
      <span className="dim">
        · <span className="mono">{repo}/{name}</span>
        {branch && (
          <>
            {" "}
            on <span className="mono">{branch}</span>
          </>
        )}
      </span>
    </p>
  );
}
