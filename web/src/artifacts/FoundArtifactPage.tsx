import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { type ArtifactFoundDetail, getArtifactFound } from "../api";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { Button, buttonClass } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { ErrorCard } from "../ui/ErrorCard";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { Keys } from "../ui/Keys";
import { Loading } from "../ui/Loading";
import { Segment, Segmented } from "../ui/Segmented";
import { Time } from "../ui/Time";
import { useTitle } from "../ui/useTitle";
import { ArtifactBody, ArtifactStage } from "./ArtifactBody";
import { ArtifactEmpty } from "./ArtifactEmpty";
import { ArtifactFrame } from "./ArtifactFrame";
import { ArtifactHead } from "./ArtifactHead";
import { ArtifactNote } from "./ArtifactNote";
import { ArtifactTitleLine, ArtifactTitleRef } from "./ArtifactTitleLine";
import { ArtifactTools, ArtifactToolsEnd } from "./ArtifactTools";
import { ArtifactView } from "./ArtifactView";
import { filesWords, widths } from "./shown";

// FoundArtifactPage is an artifact ws found in a repo (FoundArtifacts.tsx),
// running in a frame that fills the page, from the repo's files:
// /artifacts/in/<its folder's name>.
export function FoundArtifactPage() {
  const name = `${(useParams()["*"] ?? "").replace(/\/+$/, "")}/`;
  const navigate = useNavigate();
  const { value: d, error, reload } = useLoaded<ArtifactFoundDetail>(() => getArtifactFound(name), [name], { keep: true });
  useTitle(d ? d.title : name);
  const [width, setWidth] = useState(widths[0].key);
  const [run, setRun] = useState(0); // reload: a new frame
  const again = () => {
    reload();
    setRun((x) => x + 1);
  };
  useKeys({
    w: () => setWidth((w) => widths[(widths.findIndex((x) => x.key === w) + 1) % widths.length].key),
    r: again,
    O: () => d && window.open(d.frame, "_blank", "noopener,noreferrer"),
    Escape: () => navigate("/artifacts"),
  });

  if (error && !d) {
    return (
      <div className="page narrow">
        <ErrorCard error={error} retry={reload} title={`Couldn't open ${name}`} />
        <p>
          <Link to="/artifacts">Back to Artifacts</Link>
        </p>
      </div>
    );
  }
  if (!d) return <Loading what={`Opening ${name}…`} />;
  const px = widths.find((w) => w.key === width)?.px ?? 0;
  const page = d.files.some((f) => f.path === "index.html");
  const there = d.promoted ? d.all.find((v) => v.n === d.promoted?.version) : d.all[d.all.length - 1];

  return (
    <ArtifactView>
      <ArtifactHead>
        <ArtifactTitleLine back={{ to: "/artifacts", label: "Back to Artifacts" }} title={d.title}>
          <Chip tone="done">
            <Icon name="check-circle" size={11} />
            Promoted
          </Chip>
          <ArtifactTitleRef text={d.where} title="Copy where it lives" />
        </ArtifactTitleLine>
        <ArtifactNote dim>
          <Icon name="branch" size={12} /> It lives in {d.label}, at <span className="mono">{d.where}</span>
          {d.promoted ? (
            <>
              : v{d.promoted.version}, promoted <Time iso={d.promoted.at} as="relative" /> by {d.promoted.by}
            </>
          ) : d.changedBy ? (
            <>
              , last changed <Time iso={d.changed} as="relative" /> by {d.changedBy}
            </>
          ) : null}
          . {filesWords(d.files.length, d.size)}. It isn't on this machine: it runs from the repo's files.
        </ArtifactNote>
        {there?.note && (
          <ArtifactNote>
            <Inline text={there.note} />
          </ArtifactNote>
        )}
        <ArtifactTools>
          <Segmented value={width} onChange={setWidth} aria-label="Width">
            {widths.map((w) => (
              <Segment key={w.key} value={w.key} title={w.title}>
                {w.label}
              </Segment>
            ))}
          </Segmented>
          <ArtifactToolsEnd>
            <Button variant="ghost" size="small" onClick={again} disabled={!page} title="Run it again, as the repo has it now">
              <Icon name="rotate" size={13} /> Reload <Keys k="r" />
            </Button>
            {page && (
              <a className={buttonClass({ variant: "ghost" })} href={d.frame} target="_blank" rel="noopener noreferrer" title="In a tab of its own, still sandboxed">
                <Icon name="external" size={13} /> Open alone
              </a>
            )}
          </ArtifactToolsEnd>
        </ArtifactTools>
      </ArtifactHead>
      <ArtifactBody>
        <ArtifactStage>
          {page ? (
            <ArtifactFrame src={d.frame} title={d.title} px={px} frameKey={`${d.version}#${run}`} />
          ) : (
            <ArtifactEmpty icon="alert" title="There's no file to run">
              {d.where} has no index.html.
            </ArtifactEmpty>
          )}
        </ArtifactStage>
      </ArtifactBody>
    </ArtifactView>
  );
}
