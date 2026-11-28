import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { DockBody } from "../chrome/DockBody";
import { useChromeStore } from "../chrome/store";
import { useTabNames } from "../chrome/tabNames";
import { PluginFrame } from "../plugins/PluginFrame";
import { viewBase } from "../plugins/paths";
import { usePlugins } from "../workspace/store";
import css from "./PluginPane.module.css";

type PluginPaneProps = {
  // Its tab's address, which is the app's own page's: /x/jira/new.
  to: string;
  name: string;
  // Where in the app: a path, and its query.
  at: string;
};

// PluginPane is a plugin's app beside the page: what one of its actions
// opened there (where: beside), or a link to its pages peeked at. Where
// the app goes within itself, the tab goes with it (it's the same tab,
// at the app's new address); a link it follows out of itself goes in the
// page's place. Its tab's called what the app says it's showing.
export function PluginPane({ to, name, at }: PluginPaneProps) {
  const navigate = useNavigate();
  const plugin = usePlugins()?.find((p) => p.name === name);
  const tab = useRef(to);
  tab.current = to;
  const go = useCallback(
    (path: string, replace: boolean) => {
      const base = viewBase(name);
      if (path === base || path.startsWith(`${base}/`) || path.startsWith(`${base}?`)) useChromeStore.getState().open(path, { inPlaceOf: tab.current });
      else navigate(path, { replace });
    },
    [name, navigate],
  );
  // What the app says it's showing names the tab, at whatever address the
  // tab's come to.
  const [title, setTitle] = useState<string>();
  useEffect(() => {
    if (title) useTabNames.getState().say(to, title);
  }, [to, title]);
  return (
    <DockBody label={plugin?.views[0]?.title ?? name} className={css.body}>
      <PluginFrame name={name} beside={{ at, go, title: setTitle }} />
    </DockBody>
  );
}
