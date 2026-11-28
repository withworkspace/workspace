import type { PutAwayItem } from "../../api";
import { Banner } from "../../ui/Banner";
import { Button } from "../../ui/Button";
import { Spinner } from "../../ui/Spinner";
import css from "./MissionPuttingAway.module.css";

// MissionPuttingAway is what archiving the mission left to do behind:
// its capsules, burned in a moment (or being burned), and any it
// couldn't, with why, to try again or leave as they are.
export function MissionPuttingAway({
  items,
  onRetry,
  onLeave,
}: {
  items: PutAwayItem[];
  onRetry: (it: PutAwayItem) => void;
  onLeave: (it: PutAwayItem) => void;
}) {
  if (!items.length) return null;
  const failed = items.filter((it) => it.state === "failed");
  const going = items.filter((it) => it.state !== "failed");
  return (
    <div className={css.list}>
      {failed.map((it) => (
        <Banner
          key={it.id}
          icon="alert"
          actions={
            <>
              <Button size="small" onClick={() => onRetry(it)}>
                Try again
              </Button>
              <Button size="small" onClick={() => onLeave(it)}>
                Leave it
              </Button>
            </>
          }
        >
          {sentence(it.doing)} failed: {it.error}
        </Banner>
      ))}
      {going.length > 0 && (
        <Banner tone="quiet" role="status">
          <Spinner size={11} /> {going.map((it) => sentence(it.doing)).join(", ")}
          {going.some((it) => it.state === "waiting") ? " in a moment" : ""}
        </Banner>
      )}
    </div>
  );
}

const sentence = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
