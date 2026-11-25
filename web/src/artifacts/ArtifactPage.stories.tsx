import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { ArtifactEmpty } from "./ArtifactEmpty";
import { ArtifactHome } from "./ArtifactHome";
import { ArtifactPromotedNote } from "./ArtifactPromotedNote";
import { ArtifactSource } from "./ArtifactSource";
import { ArtifactSourceCode } from "./ArtifactSourceCode";
import { ArtifactSourceFile, ArtifactSourceFiles } from "./ArtifactSourceFiles";
import { days, none, Page, promoted, versions } from "./samples";

// An artifact's page, from its parts: its head with one version and with
// several (the one in the repo it lives in underlined, with the press
// that puts another there); its frame at a width of its own; its source,
// file by file; and what's said where there's no frame.

const meta: Meta = { title: "Artifacts/Artifact" };
export default meta;

export const OneVersion: StoryObj = {
  render: () => <Page title="Invoice footer" versions={[{ n: 1, by: "by you", at: days(2), note: "Where the totals sit on a narrow page." }]} />,
};

export const SeveralVersions: StoryObj = {
  render: () => (
    <Page
      n={2}
      promoted
      chip={
        <Chip tone={promoted.tone}>
          <Icon name={promoted.icon} size={11} />
          {promoted.words}
        </Chip>
      }
      notes={
        <>
          <ArtifactPromotedNote at={days(1)} by="you" version={1} into={<span className="mono">records:proposals/export.md</span>} />
          <ArtifactHome where="records:artifacts/2026-11-20-where-export-goes/" repo="records" version={1} commit="501f85db6f54" put={{ n: 2, busy: false, onPress: none }} />
        </>
      }
    />
  ),
};

export const AtAWidth: StoryObj = {
  render: () => <Page width="768" />,
};

const html = [
  '<span class="hljs-meta">&lt;!doctype html&gt;</span>',
  '<span class="hljs-tag">&lt;<span class="hljs-name">html</span> <span class="hljs-attr">lang</span>=<span class="hljs-string">"en"</span>&gt;</span>',
  '<span class="hljs-tag">&lt;<span class="hljs-name">link</span> <span class="hljs-attr">rel</span>=<span class="hljs-string">"stylesheet"</span> <span class="hljs-attr">href</span>=<span class="hljs-string">"style.css"</span>&gt;</span>',
  '<span class="hljs-tag">&lt;<span class="hljs-name">main</span> <span class="hljs-attr">class</span>=<span class="hljs-string">"dialog"</span>&gt;</span>',
  '  <span class="hljs-tag">&lt;<span class="hljs-name">h1</span>&gt;</span>Export invoices<span class="hljs-tag">&lt;/<span class="hljs-name">h1</span>&gt;</span>',
  '  <span class="hljs-tag">&lt;<span class="hljs-name">button</span> <span class="hljs-attr">id</span>=<span class="hljs-string">"go"</span>&gt;</span>Export<span class="hljs-tag">&lt;/<span class="hljs-name">button</span>&gt;</span>',
  '<span class="hljs-tag">&lt;/<span class="hljs-name">main</span>&gt;</span>',
  "",
];

const files = [
  { path: "app.js", size: "170 bytes" },
  { path: "img/logo.png", size: "70 bytes" },
  { path: "index.html", size: "383 bytes" },
  { path: "style.css", size: "406 bytes" },
];

function Files({ at, onPick }: { at: string; onPick: (path: string) => void }) {
  return (
    <ArtifactSourceFiles label="v1's files" count={files.length} size="1 KB">
      {files.map((f) => (
        <ArtifactSourceFile key={f.path} path={f.path} size={f.size} current={f.path === at} page={f.path === "index.html"} onPick={() => onPick(f.path)} />
      ))}
    </ArtifactSourceFiles>
  );
}

function SourceOf({ many }: { many?: boolean }) {
  const [at, setAt] = useState("index.html");
  return (
    <ArtifactSource files={many && <Files at={at} onPick={setAt} />}>
      {at.endsWith(".png") ? (
        <ArtifactEmpty icon="file" title={`${at} isn't text`}>
          It's 70 bytes, and the page shows it where it uses it.
        </ArtifactEmpty>
      ) : (
        <ArtifactSourceCode lines={html} path={at} />
      )}
    </ArtifactSource>
  );
}

export const Source: StoryObj = {
  render: () => <Page title="Invoice footer" versions={[versions[2]]} stage={<SourceOf />} />,
};

export const SourceWithFiles: StoryObj = {
  render: () => <Page title="Export dialog" versions={[versions[2]]} stage={<SourceOf many />} />,
};

export const Stopped: StoryObj = {
  render: () => (
    <Page
      stage={
        <ArtifactEmpty icon="stop" title="Stopped" action={<Button onClick={none}>Run it</Button>}>
          The frame's unloaded, and its scripts with it.
        </ArtifactEmpty>
      }
    />
  ),
};

export const NoSuchVersion: StoryObj = {
  render: () => (
    <Page
      stage={
        <ArtifactEmpty icon="search" title="a1 has no v9" action={<a href="#latest">Its latest, v3</a>}>
          Its versions are v1, v2, v3.
        </ArtifactEmpty>
      }
    />
  ),
};
