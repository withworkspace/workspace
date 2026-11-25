import { Markdown } from "../markdown/LazyMarkdown";

// PRDescription is a pull request's overview: what it says it does.
export function PRDescription({ body }: { body?: string }) {
  return (
    <>
      <h2>What it says it does</h2>
      {body?.trim() ? (
        <div className="prose">
          <Markdown text={body} />
        </div>
      ) : (
        <p className="dim">It has no description.</p>
      )}
    </>
  );
}
