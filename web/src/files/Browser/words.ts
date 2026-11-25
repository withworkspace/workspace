import type { TreeFile } from "../../api";
import type { IconName } from "../../ui/Icon";

// What the explorer says of a file: how it's changed since the branch
// point, how big it is, and what kind of file it is.

export type FileStatus = NonNullable<TreeFile["status"]>;

export const statusIcon: Record<string, IconName> = { added: "plus", deleted: "minus", renamed: "arrow-right", copied: "copy" };
export const statusWords: Record<string, string> = { added: "Added", modified: "Modified", deleted: "Deleted", renamed: "Renamed", copied: "Copied", type: "Changed type" };

export const images = /\.(png|jpe?g|gif|webp|svg|ico|avif)$/i;
export const isMarkdown = (path: string) => /\.(md|markdown|mdx)$/i.test(path);

export const sizeWords = (n: number) => (n < 1024 ? `${n} B` : n < 1024 * 1024 ? `${(n / 1024).toFixed(n < 10240 ? 1 : 0)} KB` : `${(n / 1024 / 1024).toFixed(1)} MB`);

// A branch as it's said: origin/main is main.
export const branchName = (ref: string) => ref.replace(/^origin\//, "");
