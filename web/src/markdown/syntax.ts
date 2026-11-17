import { mathFromMarkdown, mathToMarkdown } from "mdast-util-math";
import { math } from "micromark-extension-math";
import type { Code, Construct, Extension, State, TokenizeContext, Tokenizer } from "micromark-util-types";
import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import type { PluggableList, Processor } from "unified";

// What markdown is here, beyond CommonMark, for the reader and the editor
// alike, so they agree on where every block starts and ends:
// GitHub-flavoured (tables, task lists, strikethrough), front matter (YAML
// between --- at the top of a file), and maths as GitHub writes it ($…$ in
// a line, $$…$$ on lines of its own).
export const syntax: PluggableList = [remarkGfm, [remarkFrontmatter, ["yaml"]], remarkMaths];

// remarkMaths is remark-math with GitHub's rule for single dollars, which
// are in prose more often as money and shell variables than as maths:
// "$5 to $10" and "$HOME and $PATH" stay as they're written.
function remarkMaths(this: Processor) {
  const data = this.data() as { micromarkExtensions?: Extension[]; fromMarkdownExtensions?: unknown[]; toMarkdownExtensions?: unknown[] };
  const ext = math();
  const dollar = 36;
  const text = (ext.text as Record<number, Construct>)[dollar];
  (data.micromarkExtensions ??= []).push({ ...ext, text: { [dollar]: guarded(text) } });
  (data.fromMarkdownExtensions ??= []).push(mathFromMarkdown());
  (data.toMarkdownExtensions ??= []).push(mathToMarkdown());
}

// guarded is maths in a line, refused where GitHub wouldn't read it as
// maths: between single dollars, what's inside can't start or end with a
// space, and a letter or digit can't come right before the first dollar
// or right after the last.
function guarded(inner: Construct): Construct {
  const wordy = (code: Code) => code !== null && code > 0 && /[\p{L}\p{N}]/u.test(String.fromCodePoint(code));
  const tokenize: Tokenizer = function (this: TokenizeContext, effects, ok, nok) {
    const self = this;
    const before = self.previous;
    const after: State = (code: Code) => {
      const last = self.events[self.events.length - 1];
      const raw = last ? self.sliceSerialize(last[1]) : "";
      const single = raw.startsWith("$") && !raw.startsWith("$$");
      if (single && (/^\$\s|\s\$$/.test(raw) || wordy(before) || wordy(code))) return nok(code);
      return ok(code);
    };
    return effects.attempt(inner, after, nok);
  };
  return { name: "mathTextGuarded", tokenize, previous: inner.previous };
}
