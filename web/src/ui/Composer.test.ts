import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Composer } from "./Composer";

// Composer draws one box around its words and what's pressed. These are
// what each layout has in it, and what reaches the text box.
const html = (props: Partial<Parameters<typeof Composer>[0]> = {}, children?: string) =>
  renderToStaticMarkup(h(Composer, { value: "", onChange: () => {}, onSend: () => {}, "aria-label": "Reply", ...props }, children));

describe("Composer", () => {
  it("names its text box, and says what it's given of it", () => {
    const out = html({ placeholder: "Reply…", rows: 2, value: "Yes" });
    expect(out).toMatch(/<textarea [^>]*aria-label="Reply"/);
    expect(out).toMatch(/<textarea [^>]*placeholder="Reply…"/);
    expect(out).toMatch(/<textarea [^>]*rows="2"/);
    expect(out).toContain(">Yes</textarea>");
  });

  it("says it's being sent on the box, and only then", () => {
    expect(html({ busy: true })).toMatch(/<textarea [^>]*aria-busy="true"/);
    expect(html()).not.toContain("aria-busy");
  });

  it("has a bar under the words, with the keys that send before what's pressed", () => {
    const out = html({ hint: "No changes." }, "Send");
    expect(out).toMatch(/<\/textarea><div[^>]*><span[^>]*>No changes\.<\/span><span class="[^"]*"><kbd>[^<]*⏎<\/kbd><\/span>Send<\/div>/);
  });

  it("shows ⏎ alone where that's what sends", () => {
    expect(html({ send: "enter" })).toContain("<kbd>⏎</kbd>");
  });

  it("shows the key you've chosen where it isn't told one: ⌘⏎, till you choose", () => {
    expect(html()).toMatch(/<kbd>(⌘|Ctrl\+)⏎<\/kbd>/);
  });

  it("has what's pressed beside the words, and no bar or keys of its own", () => {
    const out = html({ layout: "beside", send: "enter" }, "Ask");
    expect(out).toMatch(/<\/textarea>Ask<\/div>$/);
    expect(out).not.toContain("<kbd>");
  });

  it("has only its keys with corner, and what's held over the words", () => {
    const out = html({ layout: "corner", over: "held" }, "not drawn");
    expect(out).toMatch(/^<div[^>]*><div[^>]*>held<\/div><textarea/);
    expect(out).toMatch(/<\/textarea><span class="[^"]*"><kbd>[^<]*⏎<\/kbd><\/span><\/div>$/);
    expect(out).not.toContain("not drawn");
  });
});
