import { RunMark } from "../../runs/RunMark";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import css from "./MissionVerifyStart.module.css";

// Where a recipe a bundle says to try stands: not running (idle), being
// asked for (starting), running with nothing answering yet (running), or
// up and answering at its link (up).
export type MissionVerifyStartState = "idle" | "starting" | "running" | "up";

type MissionVerifyStartProps = {
  recipe: string;
  // task: it ends by itself (tests, shots), so it's run, not started, and
  // has no link.
  task?: boolean;
  state: MissionVerifyStartState;
  // link is where a service answers, once it's running.
  link?: string;
  // came is what a task came to the last time it ran (passed, failed,
  // stopped), shown before its button.
  came?: string;
  // where is the capsule it runs in, as it's named (demo/refunds); note,
  // anything to know before starting it (the capsule has moved on).
  where?: string;
  note?: string;
  start: () => void;
};

// MissionVerifyStart is a recipe a bundle says to try, for the user to
// start with a button rather than copy a command: Start web, then web's
// link once it answers.
export function MissionVerifyStart({ recipe, task, state, link, came, where, note, start }: MissionVerifyStartProps) {
  const verb = task ? "Run" : "Start";
  const tip = `${verb} its ${recipe} recipe, from its ws.repo.toml${where ? `, in ${where}` : ""}, while you have this mission open${note ? `. ${note}` : ""}`;
  return (
    <span className={css.start}>
      {came && (state === "idle" || state === "starting") && <RunMark task={came} title={`Last run ${came}`} />}
      {state === "idle" || state === "starting" ? (
        <Button size="small" disabled={state === "starting"} onClick={start} title={tip}>
          {state === "starting" ? <Spinner size={11} /> : <Icon name="play" size={11} />} {state === "starting" ? `${task ? "Running" : "Starting"} ${recipe}…` : `${verb} ${recipe}${came ? " again" : ""}`}
        </Button>
      ) : (
        <span className={css.going}>
          <RunMark live up={state === "up"} />
          {task ? (
            `Running ${recipe}…`
          ) : link ? (
            <a href={link} target="_blank" rel="noopener noreferrer" className={css.open} title={state === "up" ? link : `${recipe} not ready yet`}>
              {state === "up" ? `Open ${recipe}` : `${recipe} is starting`}
              <Icon name="arrow-up-right" size={11} />
            </a>
          ) : (
            `${recipe} is starting`
          )}
        </span>
      )}
      {where && <span className={css.where}>in {where}</span>}
      {note && (
        <span className={css.where} title={note}>
          <Icon name="alert" size={11} /> {note}
        </span>
      )}
    </span>
  );
}
