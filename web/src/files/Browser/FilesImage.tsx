import css from "./FilesImage.module.css";

// FilesImage is a picture that's open, on a checkerboard so what's clear
// in it shows; a click opens it larger (the viewer).
export function FilesImage({ src, path }: { src: string; path: string }) {
  return (
    <div className={css.image}>
      <img src={src} alt={path} data-viewable={path} title="Open it larger" />
    </div>
  );
}
