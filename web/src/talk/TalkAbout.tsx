import { Link } from "react-router";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import css from "./TalkAbout.module.css";
import { Text, textClass } from "../ui/Text";

// TalkAbout is what a conversation's about, in its bar: the mission it's
// a mission's, the pull request (at the lines) or the document (at the
// section) it was asked of, each leading there; and when it began.
export function TalkAbout({
  mission,
  pr,
  doc,
  at,
}: {
  mission?: { href: string; title: string };
  pr?: { href: string; pr: string; path?: string; line?: number; endLine?: number };
  doc?: { href: string; path: string; section?: string };
  at?: string;
}) {
  return (
    <div className={css.about}>
      {mission && (
        <Link to={mission.href} className={css.thing} title="The mission it's about">
          <Icon name="flag" size={11} /> {mission.title}
        </Link>
      )}
      {pr && (
        <Link to={pr.href} className={css.thing} title="The pull request it's about">
          <Icon name="pr" size={11} /> {pr.pr}
          {pr.path && (
            <Text mono>
              {" "}
              · {pr.path}:{pr.line}
              {pr.endLine && pr.endLine > (pr.line ?? 0) ? `–${pr.endLine}` : ""}
            </Text>
          )}
        </Link>
      )}
      {doc && (
        <Link to={doc.href} className={css.thing} title={`The document it's about${doc.section ? ", at the section it was asked of" : ""}`}>
          <Icon name="book" size={11} /> <Text mono>{doc.path}</Text>
          {doc.section && <span> · § {doc.section}</span>}
        </Link>
      )}
      {at && <Time iso={at} as="relative" className={textClass("dim")} />}
    </div>
  );
}
