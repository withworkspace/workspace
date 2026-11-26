import { Checkbox } from "../ui/Checkbox";
import css from "./PagePassShown.module.css";

// PagePassShown says whether a pass's findings are marked on the page's
// words.
export function PagePassShown({ shown, onShown }: { shown: boolean; onShown: (on: boolean) => void }) {
  return (
    <Checkbox className={css.shown} checked={shown} onChange={onShown} title="Highlight findings on the page">
      Show on the page
    </Checkbox>
  );
}
