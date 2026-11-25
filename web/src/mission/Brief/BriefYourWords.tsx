import { MissionBriefQuoted } from "./MissionBriefQuoted";
import { yourWords } from "./words";

// BriefYourWords are a turn of yours: the passage it quotes, then what you
// said.
export function BriefYourWords({ body }: { body: string }) {
  const { passage, said } = yourWords(body);
  if (passage === undefined) return <>{said}</>;
  return (
    <>
      <MissionBriefQuoted>{passage}</MissionBriefQuoted>
      {said}
    </>
  );
}
