import type { DocHit } from "../api";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Row, RowLine, RowMain, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { TLink } from "../ui/TLink";
import css from "./DocumentsHitRow.module.css";
import { Text } from "../ui/Text";

type DocumentsHitRowProps = {
  id: string;
  to: string; // the document, at the section found
  selected: boolean;
  onHover: () => void;
  onOpen: (e: React.MouseEvent) => void;
  onContextMenu?: (e: React.MouseEvent) => void;
  title: string;
  heading?: string; // the section that says it best
  state?: string; // where it stands
  every: boolean; // every word was found in it
  snippet: DocHit["snippet"]; // its words, those found marked
  where: string; // which space, and where in it
};

// DocumentsHitRow is a document a search of every space found: its title
// and section, where it stands, its words with those found marked, and
// which space it's in.
export function DocumentsHitRow({ id, to, selected, onHover, onOpen, onContextMenu, title, heading, state, every, snippet, where }: DocumentsHitRowProps) {
  return (
    <Row selected={selected} id={id} role="option" aria-selected={selected} onMouseMove={onHover}>
      <TLink to={to} className={rowLinkClass(css.link)} onContextMenu={onContextMenu} onClick={onOpen}>
        <Icon name="book" size={16} className={rowStatusClass()} />
        <RowMain>
          <RowLine>
            <RowTitle>
              {title}
              {heading && <Text dim> › {heading}</Text>}
            </RowTitle>
            {state && <Chip tone="quiet">{state}</Chip>}
            {!every && (
              <Chip tone="quiet" title="Not every word was found in it">
                some of the words
              </Chip>
            )}
          </RowLine>
          {snippet.length > 0 && (
            <RowLine className={css.snippet}>
              <span>{snippet.map((p, j) => (p.mark ? <mark key={j}>{p.text}</mark> : <span key={j}>{p.text}</span>))}</span>
            </RowLine>
          )}
        </RowMain>
        <Text dim className={css.where}>{where}</Text>
      </TLink>
    </Row>
  );
}
