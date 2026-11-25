import { useEffect, useMemo, useState } from "react";
import { type ArtifactSource as Read, getArtifactSource } from "../api";
import { useLoaded } from "../load";
import { escapeHTML, highlightLines, languageOf, useHighlighter } from "../review/highlight";
import { ErrorCard } from "../ui/ErrorCard";
import { Loading } from "../ui/Loading";
import { ArtifactEmpty } from "./ArtifactEmpty";
import { ArtifactSource } from "./ArtifactSource";
import { ArtifactSourceCode } from "./ArtifactSourceCode";
import { ArtifactSourceFile, ArtifactSourceFiles } from "./ArtifactSourceFiles";
import { sizeWords } from "./shown";

// Source is a version's files, as they were written: its page, and for
// one with more, a list of them beside it to read another. A file that
// isn't text (an image, a font) says what it is.
export function Source({ id, n }: { id: string; n: number }) {
  const [file, setFile] = useState("");
  // Another version opens at its page.
  useEffect(() => setFile(""), [id, n]);
  const { value, error } = useLoaded<Read>(() => getArtifactSource(id, n, file), [id, n, file], { keep: true });
  const ready = useHighlighter();
  const lines = useMemo(() => {
    if (!value || value.binary) return [];
    const ls = value.text.split("\n");
    return ready && ls.length <= 20000 ? highlightLines(ls, languageOf(value.file)) : ls.map(escapeHTML);
  }, [value, ready]);
  if (error && !value) return <ErrorCard error={error} />;
  if (!value) return <Loading what="Reading its source…" />;
  const many = value.files.length > 1;
  return (
    <ArtifactSource
      files={
        many && (
          <ArtifactSourceFiles label={`v${value.version}'s files`} count={value.files.length} size={sizeWords(value.files.reduce((sum, f) => sum + f.size, 0))}>
            {value.files.map((f) => (
              <ArtifactSourceFile key={f.path} path={f.path} size={sizeWords(f.size)} current={f.path === value.file} page={f.path === "index.html"} onPick={() => setFile(f.path)} />
            ))}
          </ArtifactSourceFiles>
        )
      }
    >
      {error ? (
        <ErrorCard error={error} />
      ) : value.binary ? (
        <ArtifactEmpty icon="file" title={`${value.file} isn't text`}>
          It's {sizeWords(value.size)}, and the page shows it where it uses it.
        </ArtifactEmpty>
      ) : (
        <ArtifactSourceCode lines={lines} path={value.file} />
      )}
    </ArtifactSource>
  );
}
