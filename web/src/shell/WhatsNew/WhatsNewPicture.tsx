import { useState } from "react";
import { cx } from "../../ui/cx";
import css from "./WhatsNewPicture.module.css";

// WhatsNewPicture is an entry's, fetched from its release by the daemon:
// one that can't be had (offline, not published) isn't there, and the
// words say it. className is the entry's, for its shape there.
export function WhatsNewPicture({ src, alt, className }: { src: string; alt: string; className?: string }) {
  const [failed, setFailed] = useState(false);
  if (failed) return null;
  return <img className={cx(css.picture, className)} src={src} alt={alt} loading="lazy" onError={() => setFailed(true)} />;
}
