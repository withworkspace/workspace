import { Copyable } from "../../ui/Copyable";
import css from "./MissionVerifyCommand.module.css";

// MissionVerifyCommand is a command a bundle names, to copy: how to try
// it yourself, or what made a file that shows a check.
export function MissionVerifyCommand({ text }: { text: string }) {
  return <Copyable text={text} className={css.command} title="Copy command" />;
}
