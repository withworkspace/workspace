import { Link } from "react-router";
import { Banner } from "../ui/Banner";
import css from "./SpacePreviewFlight.module.css";

// SpacePreviewFlight says the document picked is changed in capsules,
// each to it as it is there: what's shown may be about to change.
export function SpacePreviewFlight({ capsules }: { capsules: { name: string; to: string }[] }) {
  return (
    <Banner tone="progress" icon="box" className={css.flight}>
      Changed in{" "}
      {capsules.map((c, i) => (
        <span key={c.name}>
          {i > 0 && ", "}
          <Link to={c.to}>{c.name}</Link>
        </span>
      ))}
      : in flight, so what's here may be about to change.
    </Banner>
  );
}
