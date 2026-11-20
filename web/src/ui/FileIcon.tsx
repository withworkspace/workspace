import { fileIconName } from "./fileIconName";
import { fileIcons } from "./fileIcons.gen";
import { Icon } from "./Icon";
import { useTheme } from "./theme";

// A file's icon, by its name: its language ("main.go", "App.tsx"), or what
// kind of file it is ("go.mod", "justfile", "logo.png"). These are Material
// Icon Theme's, in their own colours, as VS Code shows them
// (scripts/fileicons.mjs copies over the ones named there); a file of a
// kind it doesn't name gets ws's own outline of a page.

export function FileIcon({ path, size = 14, className }: { path: string; size?: number; className?: string }) {
  const { dark } = useTheme();
  const name = fileIconName(path);
  const icon = name ? fileIcons[name] : undefined;
  if (!icon) return <Icon name="file" size={size} className={`file-icon file-icon-none${className ? ` ${className}` : ""}`} />;
  return (
    <svg
      className={`file-icon${className ? ` ${className}` : ""}`}
      width={size}
      height={size}
      viewBox={`0 0 ${icon.size} ${icon.size}`}
      aria-hidden
      // The set's own SVG, copied in at build time: no one's input.
      dangerouslySetInnerHTML={{ __html: !dark && icon.light ? icon.light : icon.body }}
    />
  );
}
