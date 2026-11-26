import css from "./MissionArtifactMedia.module.css";

// MissionArtifactMedia is a mission's image or video (crew's proof, say),
// shown as it is, under its label (the page's title): an image opens in
// the viewer, to look around in; a video plays here.
export function MissionArtifactMedia({ src, video, label }: { src: string; video?: boolean; label: string }) {
  return (
    <figure className={css.media}>
      {video ? <video src={src} controls preload="metadata" aria-label={label} /> : <img src={src} alt={label} data-viewable={label} title="Enlarge" />}
    </figure>
  );
}
