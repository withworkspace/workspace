import css from "./TalkSideEmpty.module.css";

// TalkSideEmpty is the column beside a conversation before anything's
// come of it: what will be there.
export function TalkSideEmpty() {
  return <p className={css.empty}>As it goes on, this is what's come of the conversation: what's to confirm, what it's made, what it names, and its summary so far.</p>;
}
