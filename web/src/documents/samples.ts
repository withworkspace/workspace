import type { DocHit } from "../api";

// What the Documents stories draw: times before the stories' clock, and a
// search's words with those found marked.

export const ago = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString();
export const days = (n: number) => ago(n * 24 * 60);

export const records = { label: "Records", color: "#00d7af", repo: "records" };
export const api = { label: "API", color: "#5f87ff", repo: "api" };

export const snippet = (before: string, found: string, after: string): DocHit["snippet"] => [{ text: before }, { text: found, mark: true }, { text: after }];
