import { Link } from "react-router";
import type { UpdatesInfo } from "../api";
import { shortVersion } from "../shell/version";
import { Button } from "../ui/Button";
import { Callout } from "../ui/Callout";
import { Copyable } from "../ui/Copyable";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { humanError } from "../ui/strings";
import { Time } from "../ui/Time";
import { SettingsFact, SettingsFacts } from "./SettingsFacts";
import css from "./SettingsUpdates.module.css";
import type { Standing } from "./standing";
import { Text, textClass } from "../ui/Text";

type SettingsUpdatesProps = {
  // What the daemon last found of ws's releases, and where this ws stands
  // against the latest.
  up: UpdatesInfo;
  standing: Standing;
  // What's being done now: a look, or the upgrade.
  busy?: "look" | "upgrade";
  onLook: () => void;
  onUpgrade: () => void;
};

// SettingsUpdates is Updates, on Settings' About: what's out, over where
// it was found. A later release is said first, with the way to it (or why
// there's none here); one that's installed says what the daemon's waiting
// for before it restarts onto it.
export function SettingsUpdates({ up, standing, busy, onLook, onUpgrade }: SettingsUpdatesProps) {
  const notes = up.page && (
    <a href={up.page} target="_blank" rel="noopener noreferrer" className={cx(textClass("link"), css.notes)}>
      What's in it
      <Icon name="arrow-up-right" size={11} />
    </a>
  );
  return (
    <div className={css.updates}>
      {standing === "waiting" && (
        <Callout className={css.note} tone="info" icon="rotate" title={`ws ${up.installed} is installed`}>
          The daemon restarts onto it once nothing's at work
          {up.waits?.length ? (
            <>
              . It's waiting for:
              <ul className={css.waits}>
                {up.waits.map((w) => (
                  <li key={w}>{w}</li>
                ))}
              </ul>
            </>
          ) : (
            ", which is any moment now. "
          )}
          <Copyable text="ws daemon restart --now">
            ws daemon restart --now
          </Copyable>{" "}
          doesn't wait.
        </Callout>
      )}
      {standing === "out" && (
        <Callout
          className={css.note}
          tone="attention"
          icon="arrow-up"
          title={`ws ${up.latest} is out`}
          actions={
            <Button variant="primary" onClick={onUpgrade} disabled={!!busy}>
              {busy === "upgrade" ? <Spinner size={12} /> : <Icon name="arrow-up" size={12} />} {busy === "upgrade" ? "Upgrading…" : `Upgrade to ${up.latest}`}
            </Button>
          }
        >
          This is {shortVersion(up.version)}. {notes}
        </Callout>
      )}
      {standing === "out-built" && (
        <Callout className={css.note} tone="attention" icon="arrow-up" title={`ws ${up.latest} is out`}>
          This is {shortVersion(up.version)}, built from a clone rather than installed from a release, so it's changed where it was built: pull, and{" "}
          <Text mono>just build</Text>. {notes}
        </Callout>
      )}
      <SettingsFacts>
        <SettingsFact term="Latest release">
          {up.latest ? (
            <>
              <Text mono>{up.latest}</Text>
              {standing === "latest" && <Text dim> · this is it</Text>}
              {standing === "ahead" && <Text dim> · this ws is later</Text>}
              {(standing === "latest" || standing === "ahead") && notes && <> · {notes}</>}
            </>
          ) : (
            <Text dim>{up.error ? "Not known" : up.tried ? "None yet" : "Not looked for yet"}</Text>
          )}
          {up.error && (
            <div className={css.error}>
              <Icon name="alert" size={12} /> The last look didn't get an answer: {humanError(up.error)}
            </div>
          )}
        </SettingsFact>
        <SettingsFact term="Last looked" className={css.looked}>
          <span>
            {up.checked ? (
              <Time iso={up.checked} as="relative" />
            ) : up.tried ? (
              <Text dim>
                Tried <Time iso={up.tried} as="relative" />, with no answer
              </Text>
            ) : (
              <Text dim>Never</Text>
            )}
            <Text dim> · {up.checks ? "the daemon looks once a day" : "the daemon doesn't look by itself"}</Text>
          </span>
          <Button size="small" onClick={onLook} disabled={!!busy} title="Ask now for the latest release of ws, and of each plugin installed from a repo">
            {busy === "look" ? <Spinner size={12} /> : <Icon name="rotate" size={12} />} {busy === "look" ? "Looking…" : "Check now"}
          </Button>
        </SettingsFact>
        {up.plugins > 0 && (
          <SettingsFact term="Plugins">
            <Link to="/settings/plugins" className={textClass("link")}>
              {up.plugins} {up.plugins === 1 ? "plugin has" : "plugins have"} a later release
            </Link>
          </SettingsFact>
        )}
      </SettingsFacts>
    </div>
  );
}
