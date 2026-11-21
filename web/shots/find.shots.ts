// Find, opened over the missions and asked something: FEATURES.md's picture
// of it, and the way to it for anyone to watch.
import { defineFlow } from "shotta";

export default defineFlow({
  shows: "Opening Find over the missions and asking it for rounding",
  steps: [
    { goto: "/" },
    {
      annotate: "Ask Find for rounding",
      steps: [
        // mod+K opens Find, wherever the page's focus is.
        { press: "ControlOrMeta+k" },
        // Typed, not clicked into: Find takes the focus as it opens, and a
        // pointer over the page lights up the row under it.
        { type: "rounding" },
        { shot: "find", viewport: true, shows: "Find, asked for rounding: a capsule, a document, what was said, and crew" },
      ],
    },
  ],
});
