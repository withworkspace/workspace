import { useState } from "react";
import { Icon } from "../ui/Icon";
import { Status, statusClass } from "../ui/Status";
import { Focus } from "./Focus";
import { FocusRow } from "./FocusRow";
import { pins as samplePins, type SamplePin } from "./samples";

type SamplePinsProps = {
  path: string;
  start?: SamplePin[];
  // Drawn mid-drag, for a picture: this pin is being dragged, to before
  // that one.
  frozen?: { dragging: string; before: string };
};

// SamplePins is Focus for the stories: the sample pins, which unpin and
// drag into order as the app's do. It stands in for shell/focus.tsx's
// FocusSection, which keeps the pins with the daemon and will draw these
// same parts.
export function SamplePins({
  path,
  start = samplePins,
  frozen,
}: SamplePinsProps) {
  const [pins, setPins] = useState(start);
  const [moving, setMoving] = useState<string>();
  const [over, setOver] = useState<{ name: string; after: boolean }>();
  const reset = () => {
    setMoving(undefined);
    setOver(undefined);
  };
  const drop = () => {
    if (moving && over && over.name !== moving) {
      const rest = pins.filter((p) => p.name !== moving);
      const at =
        rest.findIndex((p) => p.name === over.name) + (over.after ? 1 : 0);
      rest.splice(
        at,
        0,
        pins.find((p) => p.name === moving)!,
      );
      setPins(rest);
    }
    reset();
  };
  return (
    <Focus
      count={pins.length}
      onDragOver={(e) => moving && e.preventDefault()}
      onDrop={(e) => {
        e.preventDefault();
        drop();
      }}
    >
      {pins.map((p) => (
        <FocusRow
          key={p.name}
          to={p.to}
          title={p.title}
          lead={
            p.status ? (
              <Status s={p.status} iconOnly />
            ) : (
              <span className={statusClass("quiet")}>
                <Icon name={p.icon ?? "pin"} />
              </span>
            )
          }
          on={path === p.to || path.startsWith(`${p.to}/`)}
          onUnpin={() => setPins(pins.filter((x) => x.name !== p.name))}
          dragging={(frozen?.dragging ?? moving) === p.name}
          drop={
            frozen
              ? frozen.before === p.name
                ? "before"
                : undefined
              : over?.name === p.name && moving !== p.name
                ? over.after
                  ? "after"
                  : "before"
                : undefined
          }
          onDragStart={(e) => {
            e.dataTransfer.effectAllowed = "move";
            e.dataTransfer.setData("text/plain", p.name);
            setMoving(p.name);
          }}
          onDragOver={(e) => {
            if (!moving) return;
            const box = e.currentTarget.getBoundingClientRect();
            setOver({
              name: p.name,
              after: e.clientY > box.top + box.height / 2,
            });
          }}
          onDragEnd={reset}
        />
      ))}
    </Focus>
  );
}
