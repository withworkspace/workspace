import { Text } from "../../ui/Text";
// MissionCheckingLine is what ws is running while it checks a step, in a
// line, for a list's row: what it is, the command, how long it's been
// going, and the last line it printed (judge: the judge's turn, reading
// checks, with no command and no line).
export function MissionCheckingLine({ title, judge, command, elapsed, line }: { title: string; judge?: boolean; command?: string; elapsed: string; line?: string }) {
  return (
    <>
      {title}
      {!judge ? (
        <>
          : <code>{command}</code>
        </>
      ) : null}{" "}
      · {elapsed}
      {!judge && line ? <Text dim> · {line}</Text> : null}
    </>
  );
}
