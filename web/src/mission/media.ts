// mediaKind is what a mission's artifact is shown as, by its name: an
// image or a video (crew's proof, say), which the artifact page shows as
// it is, or undefined for one that's read. The extensions are ws's
// (MediaType in internal/ops/documents.go).
export function mediaKind(name: string): "image" | "video" | undefined {
  if (/\.(png|jpe?g|gif|webp|svg|avif)$/i.test(name)) return "image";
  if (/\.(mp4|m4v|webm|mov)$/i.test(name)) return "video";
  return undefined;
}
