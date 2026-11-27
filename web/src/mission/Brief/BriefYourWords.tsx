import { RichText } from "../../ui/RichText";
import { MissionBriefQuoted } from "./MissionBriefQuoted";
import { yourWords } from "./words";

// BriefYourWords are a turn of yours: the passage it quotes, then what you
// said, as it reads (RichText).
export function BriefYourWords({ body }: { body: string }) {
  const { passage, said } = yourWords(body);
  if (passage === undefined) return <RichText text={said} />;
  return (
    <>
      <MissionBriefQuoted>{passage}</MissionBriefQuoted>
      <RichText text={said} />
    </>
  );
}
