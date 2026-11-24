import { type ReactNode, useState } from "react";
import { Button, buttonClass } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Segment, Segmented } from "../ui/Segmented";
import { Spinner } from "../ui/Spinner";
import { bare } from "./runRows";
import css from "./RunPreview.module.css";

// RunPreview is a service drawn in a frame, as an app's page draws one
// (apps/AppPage.tsx): a slim bar over a card. The bar loads it again,
// says where it's reached, sets how wide it's drawn (as a phone's or a
// tablet's, or all there's room for), and opens it in a tab of its own.
//
// It's framed once it answers (up). Until it does, the card says so over
// what it has said so far (waiting, the page's to give: its output).
// Started again, it's another process (key): the frame loads afresh.

export type PreviewWidth = "fill" | "tablet" | "phone";

const widths: Record<PreviewWidth, number | undefined> = { fill: undefined, tablet: 768, phone: 390 };

export function RunPreview({ link, title, up, port, frameKey, waiting }: { link: string; title: string; up: boolean; port: number; frameKey: string; waiting?: ReactNode }) {
  const [width, setWidth] = useState<PreviewWidth>("fill");
  const [loads, setLoads] = useState(0);
  const px = widths[width];
  return (
    <div className={css.preview}>
      <div className={css.bar}>
        <Button variant="ghost" size="small" square disabled={!up} onClick={() => setLoads((n) => n + 1)} title="Load it again">
          <Icon name="rotate" size={13} />
        </Button>
        <a className={css.address} href={link} target="_blank" rel="noopener noreferrer" title={`${link}: open it in a tab of its own`}>
          <span className={cx(css.dot, up && css.up)} aria-hidden />
          <span className={css.host}>{bare(link)}</span>
        </a>
        <Segmented size="small" className={css.widths} value={width} onChange={setWidth} aria-label="How wide it's drawn">
          <Segment value="fill" title="As wide as there's room for">
            Fill
          </Segment>
          <Segment value="tablet" title="As a tablet's, 768 wide">
            Tablet
          </Segment>
          <Segment value="phone" title="As a phone's, 390 wide">
            Phone
          </Segment>
        </Segmented>
        <a className={buttonClass({ variant: "ghost", size: "small", square: true })} href={link} target="_blank" rel="noopener noreferrer" title="Open it in a tab of its own" aria-label="Open it in a tab of its own">
          <Icon name="external" size={13} />
        </a>
      </div>
      <div className={cx(css.card, px !== undefined && css.sized)}>
        {up ? (
          <iframe key={`${frameKey}-${loads}`} src={link} title={title} className={css.frame} style={px !== undefined ? { width: px } : undefined} />
        ) : (
          <div className={css.waiting}>
            <p>
              <Spinner size={13} /> Nothing's answering on :{port} yet: it's shown here once it does.
            </p>
            {waiting}
          </div>
        )}
      </div>
    </div>
  );
}
