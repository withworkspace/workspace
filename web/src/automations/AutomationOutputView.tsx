import { type AutomationOutput, automationOutputFrameURL } from "../api";
import { Markdown } from "../markdown/Markdown";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Text } from "../ui/Text";
import { Time } from "../ui/Time";
import css from "./AutomationOutputView.module.css";

// AutomationOutputView is what an automation shows, as one version of it:
// markdown (its charts drawn), or an HTML page in a sandboxed frame, as an
// artifact's runs. Over it, which version it is, when it was written, and
// the way to the one before or after (onVersion), when there are others.
// It's read-only: the automation writes it, whole, each run.
export function AutomationOutputView({
  name,
  output,
  versions,
  onVersion,
}: {
  name: string;
  output: AutomationOutput;
  versions: number[];
  onVersion?: (version: number) => void;
}) {
  const at = versions.indexOf(output.version);
  const newer = at > 0 ? versions[at - 1] : undefined;
  const older = at >= 0 && at < versions.length - 1 ? versions[at + 1] : undefined;
  return (
    <div className={css.output}>
      <div className={css.head}>
        <Text dim>
          Written by <b>{name}</b> <Time iso={output.at} /> · version {output.version}
          {versions.length > 1 && ` of the last ${versions.length}`}
        </Text>
        {onVersion && versions.length > 1 && (
          <span className={css.versions}>
            <Button variant="ghost" size="small" disabled={older === undefined} onClick={() => older && onVersion(older)} aria-label="The version before">
              <Icon name="arrow-left" size={11} /> Older
            </Button>
            <Button variant="ghost" size="small" disabled={newer === undefined} onClick={() => newer && onVersion(newer)} aria-label="The version after">
              Newer <Icon name="arrow-right" size={11} />
            </Button>
          </span>
        )}
      </div>
      {output.format === "html" ? (
        <iframe className={css.frame} src={automationOutputFrameURL(name, output.version)} sandbox="allow-scripts" referrerPolicy="no-referrer" title={`${name}'s page`} />
      ) : (
        <div className={css.body}>
          <Markdown text={output.body} />
        </div>
      )}
    </div>
  );
}
