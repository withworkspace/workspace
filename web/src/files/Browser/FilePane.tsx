import { useEffect, useMemo, useState } from "react";
import { Link, useLocation } from "react-router";
import { type FileView, getFile, rawURL, type TreeFile } from "../../api";
import { useLoaded } from "../../load";
import { Markdown } from "../../markdown/LazyMarkdown";
import { getNavNow } from "../../review/nav";
import { buttonClass } from "../../ui/Button";
import { Callout } from "../../ui/Callout";
import { Copyable } from "../../ui/Copyable";
import { Empty } from "../../ui/Empty";
import { ErrorCard } from "../../ui/ErrorCard";
import { Icon } from "../../ui/Icon";
import { keyProps } from "../../ui/keyHint";
import { Keys } from "../../ui/Keys";
import { Loading } from "../../ui/Loading";
import { Segment, Segmented } from "../../ui/Segmented";
import { FileCode } from "./FileCode";
import { FilesChangeLegend } from "./FilesChangeLegend";
import { FilesFile, filesFileNoteClass } from "./FilesFile";
import { FilesFileHead } from "./FilesFileHead";
import { FilesFileMeta } from "./FilesFileMeta";
import { FilesFilePath } from "./FilesFilePath";
import { FilesImage } from "./FilesImage";
import { FilesMarkdown } from "./FilesMarkdown";
import { branchName, images, isMarkdown, sizeWords } from "./words";

// FilePane is the open file: where it is, how it's changed, and its text
// with the changed lines marked (or rendered, for markdown; shown, for an
// image).
export function FilePane({
  api,
  path,
  entry,
  baseRef,
  updated,
  preview,
  setPreview,
  ready,
  reviewTo,
  docTo,
  onBack,
  onReveal,
  onDefinition,
  onBackFromDefinition,
}: {
  api: string;
  path: string;
  entry?: TreeFile;
  baseRef: string;
  updated: string;
  preview: boolean;
  setPreview: (fn: (p: boolean) => boolean) => void;
  ready: boolean;
  reviewTo?: string;
  docTo?: string; // where it opens as a document, for markdown
  onBack?: () => void;
  onReveal: (folder: string) => void;
  onDefinition: (path: string, line: number) => void;
  onBackFromDefinition: () => boolean;
}) {
  // What's read is shown only while it's the file that's open: its text
  // mustn't be drawn as another's (its images read from the wrong folder)
  // for the moment between picking one and reading it.
  const [tries, setTries] = useState(0);
  const { value: read, error } = useLoaded<FileView>(() => getFile(api, path), [api, path, updated, tries], { keep: true });
  const file = read?.path === path ? read : undefined;

  // The commit its names are read at: the files as they are now. Asked
  // again each time the file's read again (crew at work in it). A file
  // deleted since its base shows as it was, so has none.
  const [rev, setRev] = useState<string>();
  const navigable = !!file && !file.deleted && !file.binary && !file.tooLarge && file.lines.length > 0;
  useEffect(() => {
    if (!navigable) return setRev(undefined);
    let live = true;
    getNavNow(api).then(
      (r) => live && setRev(r),
      () => live && setRev(undefined),
    );
    return () => {
      live = false;
    };
  }, [api, read, navigable]);

  // Where the file is, for the reader: a link in it to another file opens
  // that one here, in the same tree.
  const location = useLocation();
  const place = useMemo(
    () => ({
      api,
      path,
      to: (p: string) => {
        const next = new URLSearchParams(location.search);
        next.set("path", p);
        next.delete("only");
        return `${location.pathname}?${next}`;
      },
    }),
    [api, path, location.pathname, location.search],
  );
  const md = isMarkdown(path);
  const image = images.test(path);
  const base = branchName(baseRef);
  const text = !!file && !file.binary && !file.tooLarge;
  const size = !file ? undefined : image && !file.deleted ? (
    sizeWords(file.size)
  ) : !image && text ? (
    <>
      {file.lines.length} {file.lines.length === 1 ? "line" : "lines"}
      {file.size > 0 && ` · ${sizeWords(file.size)}`}
    </>
  ) : undefined;
  const has = (kind: "added" | "modified" | "deleted") => !!file?.changes.some((c) => c.kind === kind);
  return (
    <FilesFile>
      <FilesFileHead
        onBack={onBack}
        path={<FilesFilePath path={path} onReveal={onReveal} />}
        size={size}
        tools={
          <>
            {md && text && (
              <Segmented value={preview ? "rendered" : "text"} onChange={(v) => setPreview(() => v === "rendered")} aria-label="Markdown">
                <Segment value="rendered" title="Rendered" keys="m">
                  Rendered
                </Segment>
                <Segment value="text" title="As text" keys="m">
                  Text
                </Segment>
              </Segmented>
            )}
            {docTo && text && !file.deleted && (
              <Link to={docTo} className={buttonClass({ size: "small" })} {...keyProps("⏎", "Edit it as a page: cells, questions and passes, and saved back to the repo when you say")}>
                <Icon name="pencil" size={12} /> Edit as a page <Keys k="⏎" />
              </Link>
            )}
            {reviewTo && entry?.status && entry.status !== "added" && (
              <Link to={reviewTo} className={buttonClass({ size: "small" })} {...keyProps("v", "Review the capsule's changes")}>
                <Icon name="eye" size={12} /> Changes
              </Link>
            )}
          </>
        }
      />
      <FilesFileMeta
        status={entry?.status}
        base={base}
        oldPath={entry?.oldPath}
        uncommitted={entry?.uncommitted}
        legend={file && !image && file.changes.length > 0 && !(md && preview) && <FilesChangeLegend added={has("added")} modified={has("modified")} deleted={has("deleted")} />}
      />
      {error && <ErrorCard className={filesFileNoteClass} title="Couldn't open this file" error={error} retry={() => setTries((n) => n + 1)} />}
      {!file && !error && <Loading className={filesFileNoteClass} what="Opening it…" />}
      {file && file.deleted && (
        <Callout className={filesFileNoteClass} tone="danger" icon="minus" title={`Deleted since it left ${base}`}>
          This is how it was there.
        </Callout>
      )}
      {file &&
        (image && !file.deleted ? (
          <FilesImage src={rawURL(api, path)} path={path} />
        ) : file.binary ? (
          <Empty className={filesFileNoteClass} icon="file" title="A binary file">
            {sizeWords(file.size)}: there's no text to show.
          </Empty>
        ) : file.tooLarge ? (
          <Empty className={filesFileNoteClass} icon="file" title="Too large to show">
            It's {sizeWords(file.size)}. Open it in an editor: <Copyable text={path} />
          </Empty>
        ) : !file.lines.length ? (
          <Empty className={filesFileNoteClass} icon="file" title="An empty file" />
        ) : md && preview ? (
          <FilesMarkdown>
            <Markdown text={file.lines.join("\n")} at={place} />
          </FilesMarkdown>
        ) : (
          <FileCode file={file} path={path} ready={ready} api={api} rev={rev} onDefinition={onDefinition} onBackFromDefinition={onBackFromDefinition} />
        ))}
    </FilesFile>
  );
}
