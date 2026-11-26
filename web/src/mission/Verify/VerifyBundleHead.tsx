import type { Bundle } from "../../api";
import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { Copyable } from "../../ui/Copyable";
import { Inline } from "../../ui/Inline";
import { Time } from "../../ui/Time";
import { shortCommit, tally, verdictTone } from "../verifyRows";
import { MissionVerifyFold } from "./MissionVerifyFold";
import { MissionVerifyHead } from "./MissionVerifyHead";
import { MissionVerifyNote } from "./MissionVerifyNote";
import { MissionVerifySaid } from "./MissionVerifySaid";
import { MissionVerifySummary } from "./MissionVerifySummary";
import { MissionVerifyTally } from "./MissionVerifyTally";
import { VerifyTry } from "./VerifyTry";
import { stepTitle } from "./words";
import { Text } from "../../ui/Text";

// VerifyBundleHead is the open bundle: who made it and of what, whether
// it's still of its capsule as it is, what you've said of it, its summary
// (a few lines, until you ask for the rest), and, folded, what it didn't
// verify and how to try it.
export function VerifyBundleHead({ b, pick }: { b: Bundle; pick: (id?: string) => void }) {
  return (
    <MissionVerifyHead
      id={b.id}
      title={stepTitle(b)}
      current={!!b.capsule && !!b.commit && !b.stale && !b.replacedBy}
      meta={
        <>
          From {b.crew}
          {b.by ? ` (${b.by})` : ""}
          {b.own ? ", who did the work" : ", an independent verifier"}
          {b.capsule && (
            <>
              , of <Text mono>{b.capsule}</Text>
              {b.commit && (
                <>
                  {" "}
                  at{" "}
                  <Copyable text={b.commit} title="Copy commit">
                    {shortCommit(b.commit)}
                  </Copyable>
                </>
              )}
            </>
          )}
          , <Time iso={b.at} as="relative" />
        </>
      }
    >
      {b.replacedBy && (
        <MissionVerifyNote tone="quiet" icon="info" actions={<Button onClick={() => pick(b.replacedBy!)}>Open {b.replacedBy}</Button>}>
          Replaced by {b.replacedBy}.
        </MissionVerifyNote>
      )}
      {b.verdict === "approved" && (
        <MissionVerifyNote tone="done" icon="check-circle">
          You approved it
          {b.verdictAt && (
            <>
              {" "}
              <Time iso={b.verdictAt} as="relative" />
            </>
          )}
          .
        </MissionVerifyNote>
      )}
      {b.verdict === "sent back" && (
        <MissionVerifyNote
          tone="attention"
          icon="reply"
          title={
            <>
              You sent it back
              {b.verdictAt && (
                <>
                  {" "}
                  <Time iso={b.verdictAt} as="relative" />
                </>
              )}
            </>
          }
        >
          <MissionVerifySaid notes={b.notes ?? []} />
          {!b.replacedBy && <>With crew: their next bundle replaces this one.</>}
        </MissionVerifyNote>
      )}
      {b.stale && !b.replacedBy && (
        <MissionVerifyNote tone="attention" title="Stale">
          Capsule has moved on to <Text mono>{shortCommit(b.head)}</Text>. This bundle shows{" "}
          <Text mono>{shortCommit(b.commit)}</Text>.
        </MissionVerifyNote>
      )}
      <MissionVerifySummary text={b.summary} />
      <MissionVerifyTally
        replaces={
          b.replaces && (
            <>
              Replaces{" "}
              <Button variant="link" type="button" onClick={() => pick(b.replaces!)}>
                {b.replaces}
              </Button>
              .
            </>
          )
        }
      >
        {tally(b).map(([v, n]) => (
          <Chip tone={verdictTone(v)} key={v}>
            {n} {v}
          </Chip>
        ))}
      </MissionVerifyTally>
      {b.notVerified?.length ? <MissionVerifyFold title="Not verified" items={b.notVerified.map((n) => <Inline text={n} />)} /> : null}
      {b.noticed?.length ? <MissionVerifyFold title="Noticed" items={b.noticed.map((n) => <Inline text={n} />)} /> : null}
      {b.try?.length ? <MissionVerifyFold title="Try it" items={b.try.map((t) => <VerifyTry t={t} />)} /> : null}
    </MissionVerifyHead>
  );
}
