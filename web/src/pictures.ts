// A picture pasted for an agent is named, in what's written, by its path
// in full (the agent reads it there): <workspace>/.ws/images/<name>. The
// web UI draws it from where ws serves it. Mirrors internal/images.
const pasted = /[/\\]\.ws[/\\]images[/\\]([0-9a-f]{32}\.(?:png|jpg|gif|webp))$/;

// pictureURL is where a pasted picture's drawn from, if src is one.
export function pictureURL(src: string | null | undefined): string | undefined {
  const m = src ? pasted.exec(src) : null;
  return m ? `/api/images/${m[1]}` : undefined;
}

// isPicture is whether a file pasted or dropped is a picture an agent can see.
export const isPicture = (f: File) => /^image\/(png|jpeg|gif|webp)$/.test(f.type);
