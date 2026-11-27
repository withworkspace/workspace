import { type KeyLike, useSaid } from "../keymap/keymap";
import { type ComponentProps, createContext, type ReactNode, useContext } from "react";
import { Tab as AriaTab, TabList, Tabs as AriaTabs } from "react-aria-components";
import { cx } from "./cx";
import { keyHint, pointed, titleWithKeys } from "./keyHint";
import css from "./Tabs.module.css";

// The id of what the tabs change, for each Tab to say.
const Controls = createContext<string | undefined>(undefined);

// Tabs is a row of words, the one you're on underlined: the views of one
// thing (a mission's Summary, Brief, Work), one shown at a time. value is
// the one shown, and onChange is called with another when it's chosen.
// react-aria's tabs do the rest: the row is one stop for Tab, the arrows
// move along it and show each as they go, and a screen reader hears tabs.
//
// What a tab shows is the page's to draw, wherever it is on it: a
// role="tabpanel" with an aria-label, and its id here as controls, so
// the tabs say what they change.
export function Tabs<T extends string>({
  value,
  onChange,
  controls,
  className,
  "aria-label": label,
  children,
}: {
  value: T;
  onChange: (value: T) => void;
  controls?: string;
  className?: string;
  "aria-label": string;
  children: ReactNode; // Tabs
}) {
  return (
    <AriaTabs className={css.root} selectedKey={value} onSelectionChange={(key) => key !== value && onChange(String(key) as T)}>
      {/* data-tabs: what styles.css leaves out of a print. */}
      <TabList className={cx(css.tabs, className)} aria-label={label} data-tabs="">
        <Controls.Provider value={controls}>{children}</Controls.Provider>
      </TabList>
    </AriaTabs>
  );
}

// Tab is one of them: its value, and what it shows (words, an icon
// before, a count after). A title says more about it on hover. keys is
// the key that shows it, said after the title and hinted as a Button's
// is (ui/keyHint.ts). onPress is called on pressing it, as well as its
// being chosen: for the one shown, which choosing doesn't change.
export function Tab({ value, title, keys: given, onPress, className, children }: { value: string; title?: string; keys?: KeyLike; onPress?: () => void; className?: string; children: ReactNode }) {
  const controls = useContext(Controls);
  const keys = useSaid(given);
  const titled = titleWithKeys(title, keys);
  // react-aria's tab names a panel of its own, which isn't drawn: it says
  // the page's instead, and its title, which react-aria's drops. (It's a
  // div: only a tab that's a link, which these aren't, is anything else.)
  const draw = (props: ComponentProps<"div">) => <div {...props} title={titled} aria-controls={props["aria-selected"] ? controls : undefined} />;
  return (
    <AriaTab id={value} className={cx(css.tab, className)} onPress={keys || onPress ? (e) => {
        if (keys && pointed(e)) keyHint(keys);
        onPress?.();
      } : undefined} render={(props) => draw(props as ComponentProps<"div">)}>
      {children}
    </AriaTab>
  );
}
