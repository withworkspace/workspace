import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import css from "./WhatsNewShowMe.module.css";

// WhatsNewShowMe goes where a thing that's new can be seen.
export function WhatsNewShowMe({ onClick }: { onClick: () => void }) {
  return (
    <Button variant="ghost" className={css.showMe} onClick={onClick}>
      Show me <Icon name="arrow-right" size={12} />
    </Button>
  );
}
