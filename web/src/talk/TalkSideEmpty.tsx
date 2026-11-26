import css from "./TalkSideEmpty.module.css";

// TalkSideEmpty is the column beside a conversation before anything's
// come of it: what will be there.
export function TalkSideEmpty() {
  return <p className={css.empty}>Proposals to confirm, things made, mentions and a summary show here as the conversation goes on.</p>;
}
