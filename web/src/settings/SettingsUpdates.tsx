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
      Release notes<Icon name="arrow-up-right" size={11} />
    </a>
  );
  return (
    <div className={css.updates}>
      {standing === "waiting" && (
        <Callout className={css.note} tone="info" icon="rotate" title={`Workspace ${up.installed} is installed`}>
          Workspace switches over when idle
          {up.waits?.length ? (
            <>
              . Waiting for:
              <ul className={css.waits}>
                {up.waits.map((w) => (
                  <li key={w}>{w}</li>
                ))}
              </ul>
            </>
          ) : (
            ", so any moment now. "
          )}
          <Copyable text="ws daemon restart --now">
            ws daemon restart --now
          </Copyable>{" "}
          skips the wait.
        </Callout>
      )}
      {standing === "out" && (
        <Callout
          className={css.note}
          tone="attention"
          icon="arrow-up"
          title={`Workspace ${up.latest} is out`}
          actions={
            <Button variant="primary" onClick={onUpgrade} disabled={!!busy}>
              {busy === "upgrade" ? <Spinner size={12} /> : <Icon name="arrow-up" size={12} />} {busy === "upgrade" ? "Upgrading…" : `Upgrade to ${up.latest}`}
            </Button>
          }
        >
          Running {shortVersion(up.version)}. {notes}
        </Callout>
      )}
      {standing === "out-built" && (
        <Callout className={css.note} tone="attention" icon="arrow-up" title={`Workspace ${up.latest} is out`}>
          Running {shortVersion(up.version)}, built from source: pull and run <Text mono>just build</Text> to
          update. {notes}
        </Callout>
      )}
      <SettingsFacts>
        <SettingsFact term="Latest release">
          {up.latest ? (
            <>
              <Text mono>{up.latest}</Text>
              {standing === "latest" && <Text dim> · up to date</Text>}
              {standing === "ahead" && <Text dim> · yours is newer</Text>}
              {(standing === "latest" || standing === "ahead") && notes && <> · {notes}</>}
            </>
          ) : (
            <Text dim>{up.error ? "Unknown" : up.tried ? "None yet" : "Not checked yet"}</Text>
          )}
          {up.error && (
            <div className={css.error}>
              <Icon name="alert" size={12} /> Last check failed: {humanError(up.error)}
            </div>
          )}
        </SettingsFact>
        <SettingsFact term="Last checked" className={css.looked}>
          <span>
            {up.checked ? (
              <Time iso={up.checked} as="relative" />
            ) : up.tried ? (
              <Text dim>
                Failed <Time iso={up.tried} as="relative" />
              </Text>
            ) : (
              <Text dim>Never</Text>
            )}
            <Text dim> · {up.checks ? "checked daily" : "automatic checks off"}</Text>
          </span>
          <Button size="small" onClick={onLook} disabled={!!busy} title="Check for Workspace and plugin updates">
            {busy === "look" ? <Spinner size={12} /> : <Icon name="rotate" size={12} />} {busy === "look" ? "Checking…" : "Check now"}
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
