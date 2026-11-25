import { Markdown } from "../../markdown/LazyMarkdown";
import css from "./WhatsNewEntry.module.css";
import { WhatsNewPicture } from "./WhatsNewPicture";
import { WhatsNewShowMe } from "./WhatsNewShowMe";
import { WhatsNewVersion } from "./WhatsNewVersion";

type WhatsNewEntryProps = {
  // The first, large: its picture beside its words. Otherwise a card.
  lead?: boolean;
  picture?: string;
  title: string;
  // A sentence or two, in markdown.
  body: string;
  // Which release it's in, said where What's new is of several.
  release?: string;
  // Where there's somewhere to see it: going there.
  onShow?: () => void;
};

// WhatsNewEntry is one thing that's new: its picture, if it has one, and
// what it says.
export function WhatsNewEntry({ lead, picture, title, body, release, onShow }: WhatsNewEntryProps) {
  return (
    <article className={lead ? css.lead : css.card}>
      {picture && <WhatsNewPicture src={picture} alt={title} className={css.picture} />}
      <div className={css.words}>
        {release && <WhatsNewVersion>{release}</WhatsNewVersion>}
        <h3>{title}</h3>
        <Markdown text={body} />
        {onShow && <WhatsNewShowMe onClick={onShow} />}
      </div>
    </article>
  );
}
