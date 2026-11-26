import { Markdown } from "../markdown/LazyMarkdown";
import { Text } from "../ui/Text";
import { Prose } from "../ui/Prose";

// PRDescription is a pull request's overview: what it says it does.
export function PRDescription({ body }: { body?: string }) {
  return (
    <>
      <h2>Description</h2>
      {body?.trim() ? (
        <Prose>
          <Markdown text={body} />
        </Prose>
      ) : (
        <Text as="p" dim>No description.</Text>
      )}
    </>
  );
}
