import { Empty } from "../ui/Empty";
import { Keys } from "../ui/Keys";
import css from "./EmptyPane.module.css";

// EmptyPane is Beside with nothing in it here, docked: it stays as you
// go from page to page so the page's width doesn't change under you, and
// says how something comes to be in it. elsewhere: how many are open
// beside other things' pages, which come back there.
export function EmptyPane({ elsewhere = 0 }: { elsewhere?: number }) {
  return (
    <Empty icon="panel" title="Nothing beside this page" className={css.empty}>
      <p className={css.says}>{elsewhere > 0 ? `${elsewhere === 1 ? "One more is" : `${elsewhere} more are`} beside the page ${elsewhere === 1 ? "it's" : "they're"} about.` : "Beside stays here as you go from page to page."}</p>
      <dl className={css.keys}>
        <dt>
          <Keys k="C" />
        </dt>
        <dd>A new conversation</dd>
        <dt>
          <Keys k="z" />
        </dt>
        <dd>Peek at what the pointer's on</dd>
        <dt>
          <Keys k="\" />
        </dt>
        <dd>Close Beside</dd>
      </dl>
    </Empty>
  );
}
