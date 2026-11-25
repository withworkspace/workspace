import { Link } from "react-router";

// ProcessAbout is what the process is, under the section's name.
export function ProcessAbout() {
  return (
    <>
      How work goes here, in prose: the steps work takes, whose turn each is, what done means, and when it's yours. It's the team's, in the config repo, changed by commit
      like the rest of it. The planner reads it as it's written to plan a mission, and the orchestrator to know what to do when something happens. What ws holds whatever
      they read is under{" "}
      <Link to="/settings/missions#missions" className="link">
        Guarantees
      </Link>
      .
    </>
  );
}
