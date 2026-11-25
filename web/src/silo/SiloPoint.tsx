import { useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { pointSilo } from "../api";
import { humanError } from "../ui/text";
import { SiloPointAsk } from "./SiloPointAsk";

// /silo/<repo>?to=<capsule>&then=<address>: point a repo's silo at a
// capsule, then go on to where you were going. ws's router sends you here
// when you ask for a silo's run by a capsule's name and the silo is on
// another: it never switches the silo itself (two tabs would fight over
// it), so it's yours to say, here, signed in.

// onward is then, if it's somewhere of ws's router's (a name under
// .localhost): this page sends you nowhere else.
export function onward(then: string | null): string | undefined {
  if (!then) return undefined;
  try {
    const u = new URL(then);
    return (u.protocol === "http:" || u.protocol === "https:") && u.hostname.endsWith(".localhost") ? u.toString() : undefined;
  } catch {
    return undefined;
  }
}

export function SiloPoint() {
  const { repo = "" } = useParams();
  const [q] = useSearchParams();
  const to = q.get("to") ?? "";
  const then = onward(q.get("then"));
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const point = () => {
    setBusy(true);
    setError("");
    pointSilo(repo, to)
      .then(() => {
        if (then) window.location.assign(then);
        else navigate("/");
      })
      .catch((e: Error) => setError(humanError(e.message)))
      .finally(() => setBusy(false));
  };
  return <SiloPointAsk repo={repo} to={to} then={!!then} busy={busy} error={error} onPoint={point} onNotNow={() => navigate("/")} />;
}
