import { buttonClass } from "../ui/Button";
import { Icon } from "../ui/Icon";
import css from "./TalkProposalFrame.module.css";
import { Text } from "../ui/Text";

// TalkProposalFrame is a version, or a proposal, running in a small
// frame, as the artifact's page runs one: sandbox="allow-scripts", no
// allow-same-origin, so its scripts run in an origin of their own and its
// policy lets nothing leave. Open larger opens it alone, in a tab of its
// own, under the same policy.
export function TalkProposalFrame({ src, caption, title }: { src: string; caption?: string; title: string }) {
  return (
    <figure className={css.frame}>
      {caption && <Text as="figcaption" dim>{caption}</Text>}
      <iframe src={src} sandbox="allow-scripts" referrerPolicy="no-referrer" loading="lazy" title={title} />
      <a href={src} target="_blank" rel="noopener noreferrer" className={buttonClass({}, css.open)} title="Open in new tab (sandboxed)">
        <Icon name="external" size={12} /> Open larger
      </a>
    </figure>
  );
}
