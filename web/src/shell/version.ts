// shortVersion is ws's version as the menu says it. A release is as it is
// (v1.4.0, v1.4.0-rc.1). A build between releases has Go's pseudo-version,
// which is long (v0.1.1-0.20261117213809-56368bcb84bf+dirty): the release
// it's after, that it's a dev build, and its commit. The whole of it is
// the title.
export function shortVersion(v: string): string {
  const m = /^(v\d+\.\d+\.\d+)-(?:.+\.)?\d{14}-([0-9a-f]{12})(\+dirty)?$/.exec(v);
  if (m) return `${m[1]}-dev · ${m[2].slice(0, 7)}`;
  return v === "(devel)" || v === "dev" ? "dev build" : v;
}
