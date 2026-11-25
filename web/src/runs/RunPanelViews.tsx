import { Icon } from "../ui/Icon";
import { Tab, Tabs } from "../ui/Tabs";
import type { RunView } from "./runView";
import css from "./RunPanelViews.module.css";

// RunPanelViews is what's shown of a run, over it: Preview and Output as
// tabs where it's framed (shown, onChange; controls is the id of what
// they show), or Output alone, as a heading in their place, where there's
// nothing to frame (a task, a service ended or opened in a tab of its
// own). name is the run's.
export function RunPanelViews({ framed, shown, onChange, controls, name }: { framed: boolean; shown: RunView; onChange: (v: RunView) => void; controls: string; name: string }) {
  return (
    <div className={css.views}>
      {framed ? (
        <Tabs value={shown} onChange={onChange} controls={controls} className={css.tabs} aria-label={`What's shown of ${name}`}>
          <Tab value="preview" title="It, in a frame">
            <Icon name="eye" size={13} /> Preview
          </Tab>
          <Tab value="output" title="What it's printing, as it does">
            <Icon name="terminal" size={13} /> Output
          </Tab>
        </Tabs>
      ) : (
        <h3 className={css.view}>
          <Icon name="terminal" size={13} /> Output
        </h3>
      )}
    </div>
  );
}
