import { useState } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Input } from "../ui/Input";
import { Popover } from "../ui/Popover";
import css from "./MissionDrop.module.css";

// MissionDrop is giving a mission up: a quiet button that opens a line
// for why, if you want to say, and the button that drops it. Return in
// the line drops it too. It reads nothing: what dropping does is whoever
// draws it's to say (onDrop, given why, "" for nothing said). Open from
// the first is for a picture of it.
export function MissionDrop({ busy, open, onDrop }: { busy?: boolean; open?: boolean; onDrop: (why: string) => void }) {
  const [why, setWhy] = useState("");
  return (
    <Popover
      aria-label="Drop mission"
      placement="bottom end"
      defaultOpen={open}
      trigger={
        <Button variant="ghost" size="small" title="End as dropped. Reopen to undo.">
          <Icon name="x" size={12} /> Drop
        </Button>
      }
    >
      <form
        className={css.drop}
        onSubmit={(e) => {
          e.preventDefault();
          onDrop(why.trim());
        }}
      >
        <Input className={css.why} value={why} onChange={(e) => setWhy(e.target.value)} placeholder="Reason (optional)" aria-label="Reason" autoFocus disabled={busy} />
        <Button variant="danger" type="submit" disabled={busy}>
          Drop
        </Button>
      </form>
    </Popover>
  );
}
