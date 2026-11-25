import css from "./ConversationGiven.module.css";

type ConversationGivenProps = {
  // Who gave it: the page, by name.
  from?: string;
  // What it gave, each under its label: what was selected, what came with
  // the message, the page's state.
  parts: [label: string, text: string][];
};

// ConversationGiven is what a page gave the agent with something you
// said: folded, for you to look at, as it isn't your words.
export function ConversationGiven({ from, parts }: ConversationGivenProps) {
  return (
    <details className={css.given}>
      <summary>with context from {from || "the page"}</summary>
      {parts.map(([label, text]) => (
        <div key={label}>
          <div className={css.label}>{label}</div>
          {text && <pre>{text}</pre>}
        </div>
      ))}
    </details>
  );
}
