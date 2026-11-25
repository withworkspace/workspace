import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import css from "./ThingSave.module.css";
import { ThingUi, ThingUiButtons, ThingUiFailed, ThingUiTitle } from "./ThingUi";

// ThingSaveSaved is what came of a save.
export type ThingSaveSaved = {
  // The repo it lives in, and where in it.
  repo: string;
  path: string;
  // It changed what was there, and what was there had changed since.
  changed: boolean;
  merged?: boolean;
  // The capsule it was left in, where its repo takes pull requests.
  capsule?: string;
};

type ThingSaveProps = {
  // The thing, by its title or its ref.
  what: string;
  // How it's going: saving, saved (with what came of it), or failed (with why).
  saved?: ThingSaveSaved;
  why?: string;
  onAgain: () => void;
  onClose: () => void;
};

// ThingSave is a save of a plugin's thing to its repo, by the button that
// asked: how it's going, what came of it, or why it couldn't be.
export function ThingSave({ what, saved, why, onAgain, onClose }: ThingSaveProps) {
  return (
    <ThingUi role="status">
      {!saved && why === undefined && (
        <p className={css.line}>
          <Spinner size={14} /> Saving {what} to its repo…
        </p>
      )}
      {saved && (
        <>
          <p className={css.line}>
            <Icon name="check" size={14} /> {saved.capsule ? "Saved in a capsule" : saved.changed ? "Saved" : "Nothing to save"}
            <code>
              {saved.repo}:{saved.path}
            </code>
          </p>
          {saved.merged && <p className="dim">It had changed there since: what was there is in it now.</p>}
          {saved.capsule && <p className="dim">{saved.repo} takes pull requests: it's in {saved.capsule}, for you to open one from.</p>}
          {!saved.changed && !saved.capsule && <p className="dim">It's there as it is here.</p>}
        </>
      )}
      {!saved && why !== undefined && (
        <>
          <ThingUiTitle>{what} wasn't saved</ThingUiTitle>
          <ThingUiFailed>{why}</ThingUiFailed>
          <ThingUiButtons>
            <Button variant="primary" size="small" type="button" onClick={onAgain}>
              Try again
            </Button>
            <Button variant="ghost" size="small" type="button" onClick={onClose} keys="esc">
              Close
            </Button>
          </ThingUiButtons>
        </>
      )}
    </ThingUi>
  );
}
