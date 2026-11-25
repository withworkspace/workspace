import css from "./ArtifactFrom.module.css";

// ArtifactFrom is where a version came from, for one this machine hasn't
// got (made on another): its title, in quotes, or failing that its name.
export function ArtifactFrom({ from, title }: { from: string; title?: string }) {
  return (
    <span className={css.from} title={title ? `${from}: ${title}` : from}>
      {title ? `“${title}”` : from}
    </span>
  );
}
