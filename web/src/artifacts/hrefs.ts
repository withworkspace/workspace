// Where an artifact's pages are.

// artifactHref is an artifact's page, at its latest or at a version:
// /artifacts/a3, /artifacts/a3/v2.
export const artifactHref = (id: string, n?: number) => (n ? `/artifacts/${id}/v${n}` : `/artifacts/${id}`);

// foundHref is a found artifact's page: /artifacts/in/records/artifacts/2026-11-20-horizon-glow/.
export const foundHref = (name: string) => `/artifacts/in/${name.split("/").map(encodeURIComponent).join("/")}`;
