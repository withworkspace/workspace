import { Navigate, useParams } from "react-router";
import { briefHref } from "../mission/BriefTab";

// ToBrief is where the brief's own page was: its mission's Brief tab,
// which is where ws still sends you to brief one.
export function ToBrief() {
  const { slug = "" } = useParams();
  return <Navigate to={briefHref(slug)} replace />;
}
