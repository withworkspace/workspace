import { Link } from "react-router";
import { textClass } from "../ui/Text";

// ProcessAbout is what the process is, under the section's name.
export function ProcessAbout() {
  return (
    <>
      How work goes here, in prose: the steps, whose turn each is, what done means, and when it's yours. The
      team's file, in the config repo, changed by commit. The planner reads it to plan missions, and the
      orchestrator to decide what to do next. What Workspace guarantees regardless is under{" "}
      <Link to="/settings/missions#missions" className={textClass("link")}>
        Guarantees
      </Link>.
    </>
  );
}
