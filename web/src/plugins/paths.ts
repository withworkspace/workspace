// Where a plugin's things are served, in ws. Its view's address is
// /x/<name>/…; the page its app runs in (the shell) is the same path under
// frameBase; and its own HTTP (its app's files, and what its app asks it)
// is under apiBase. The server's side of these is internal/web/plugins.go.
export const viewBase = (name: string) => `/x/${name}`;
export const frameBase = (name: string) => `/plugin-frame/${name}`;
export const apiBase = (name: string) => `/plugins/${name}`;
