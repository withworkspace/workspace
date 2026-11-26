import { docAddress, type Space, type SpaceFlight } from "../api";
import { Rows } from "../ui/Rows";
import { DocumentsActivitySection } from "./DocumentsActivitySection";
import { DocumentsFlightRow } from "./DocumentsFlightRow";
import { DocumentsSpaceChange } from "./DocumentsSpaceChange";
import { DocumentsSpaceTag } from "./DocumentsSpaceTag";
import type { InSpace } from "./happening";

// InFlight is the capsules with changes to a space's documents (or to
// any space's, each with its space), each with the pull request open from
// it, and what it changes, read as it is there. flat: in a space's
// DocumentsActivityFold.
export function InFlight({ flights, spaceOf, flat }: { flights: InSpace<SpaceFlight>[]; spaceOf?: (repo: string) => Space | undefined; flat?: boolean }) {
  if (!flights.length) return null;
  return (
    <DocumentsActivitySection label="In flight" heading="In flight" count={flights.length} flat={flat} title="Document changes in capsules, with their pull requests">
      <Rows>
        {flights.map((f) => {
          const space = spaceOf?.(f.repo);
          return (
            <DocumentsFlightRow
              key={`${f.repo}:${f.capsule}`}
              capsule={f.capsule}
              mission={f.mission}
              pr={f.pr && { number: f.pr.number, title: f.pr.title, to: `/reviews/${encodeURIComponent(f.repo)}/${f.pr.number}` }}
              space={space && <DocumentsSpaceTag label={space.label} color={space.color} end />}
            >
              {f.docs.map((d) => (
                <DocumentsSpaceChange key={d.path} how={d.how} name={d.title || d.path.split("/").pop() || d.path} path={d.path} to={d.how === "removed" ? undefined : docAddress(`${f.repo}/${f.capsule}:${d.path}`)} />
              ))}
            </DocumentsFlightRow>
          );
        })}
      </Rows>
    </DocumentsActivitySection>
  );
}
