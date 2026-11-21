// A tone is what colour something says it's in: yours to do (attention),
// moving (progress), done, waiting on someone else, failing (danger), to
// know (info), or nothing to see (quiet). styles.css has a class for each,
// tone-<tone>, which sets --tone, --tone-fill and --tone-soft for whatever
// it's on to paint with: a chip, a count, a callout, a status.
export type Tone = "attention" | "progress" | "done" | "waiting" | "danger" | "info" | "quiet";

// toneClass is the class that sets a tone's colours, or nothing.
export const toneClass = (tone?: Tone): string | undefined => (tone ? `tone-${tone}` : undefined);
