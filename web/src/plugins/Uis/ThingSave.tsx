import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import css from "./ThingSave.module.css";
import { ThingUi, ThingUiButtons, ThingUiFailed, ThingUiTitle } from "./ThingUi";
import { Text } from "../../ui/Text";

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
          <Spinner size={14} /> Saving {what} to repo…
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
          {saved.merged && <Text as="p" dim>Changed in the repo since: changes merged.</Text>}
          {saved.capsule && <Text as="p" dim>{saved.repo} uses pull requests: saved in {saved.capsule} for you to open one.</Text>}
          {!saved.changed && !saved.capsule && <Text as="p" dim>Already up to date.</Text>}
        </>
      )}
      {!saved && why !== undefined && (
        <>
          <ThingUiTitle>Could not save {what}</ThingUiTitle>
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
