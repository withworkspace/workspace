import type { ReactNode } from "react";

// One small icon set, drawn on a 24px grid with a 2px round stroke and shown
// at 14 or 16px: in place of the unicode glyphs that used to do this job
// (◐ ● ✓ ↻ ↩ …), which sat on different baselines and changed with the
// font.

const c = (cx: number, cy: number, r: number, fill = false) => <circle cx={cx} cy={cy} r={r} fill={fill ? "currentColor" : "none"} />;
const p = (d: string) => <path d={d} />;
const r = (x: number, y: number, w: number, h: number, rx = 2) => <rect x={x} y={y} width={w} height={h} rx={rx} />;

export const icons = {
  check: p("M20 6 9 17l-5-5"),
  x: [p("M18 6 6 18"), p("m6 6 12 12")],
  plus: [p("M5 12h14"), p("M12 5v14")],
  minus: p("M5 12h14"),
  "arrow-left": [p("m12 19-7-7 7-7"), p("M19 12H5")],
  "arrow-right": [p("M5 12h14"), p("m12 5 7 7-7 7")],
  "arrow-down": [p("M12 5v14"), p("m19 12-7 7-7-7")],
  "arrow-up": [p("M12 19V5"), p("m5 12 7-7 7 7")],
  "arrow-up-right": [p("M7 7h10v10"), p("M7 17 17 7")],
  swap: [p("m16 3 4 4-4 4"), p("M20 7H4"), p("m8 21-4-4 4-4"), p("M4 17h16")],
  "chevron-right": p("m9 18 6-6-6-6"),
  "chevron-down": p("m6 9 6 6 6-6"),
  "chevron-up": p("m18 15-6-6-6 6"),
  reply: [p("M20 4v7a4 4 0 0 1-4 4H4"), p("m9 10-5 5 5 5")],
  rotate: [p("M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"), p("M21 3v5h-5")],
  copy: [r(8, 8, 14, 14), p("M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2")],
  search: [c(11, 11, 8), p("m21 21-4.3-4.3")],
  menu: [p("M4 6h16"), p("M4 12h16"), p("M4 18h16")],
  grip: [c(9, 6, 1, true), c(15, 6, 1, true), c(9, 12, 1, true), c(15, 12, 1, true), c(9, 18, 1, true), c(15, 18, 1, true)],
  panel: [r(3, 3, 18, 18), p("M15 3v18")],
  float: [r(3, 3, 18, 18), r(11, 11, 7, 7, 1)],
  fold: [r(3, 3, 18, 18), p("M3 16h18")],
  sliders: [p("M21 4h-7M10 4H3M21 12h-9M8 12H3M21 20h-5M12 20H3"), p("M14 2v4M8 10v4M16 18v4")],
  sun: [c(12, 12, 4), p("M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41")],
  moon: p("M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"),
  monitor: [r(2, 3, 20, 14), p("M8 21h8"), p("M12 17v4")],
  message: p("M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"),
  folder: p("M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"),
  file: [p("M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"), p("M14 2v4a2 2 0 0 0 2 2h4"), p("M16 13H8"), p("M16 17H8"), p("M10 9H8")],
  branch: [p("M6 3v12"), c(18, 6, 3), c(6, 18, 3), p("M18 9a9 9 0 0 1-9 9")],
  commit: [c(12, 12, 3), p("M3 12h6"), p("M15 12h6")],
  pr: [c(18, 18, 3), c(6, 6, 3), p("M13 6h3a2 2 0 0 1 2 2v7"), p("M6 9v12")],
  terminal: [p("m4 17 6-6-6-6"), p("M12 19h8")],
  user: [p("M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"), c(12, 7, 4)],
  sparkle: p("M12 3 13.9 9.1 20 11l-6.1 1.9L12 19l-1.9-6.1L4 11l6.1-1.9Z"),
  box: [p("M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"), p("m3.3 7 8.7 5 8.7-5"), p("M12 22V12")],
  eye: [p("M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"), c(12, 12, 3)],
  pencil: [p("M12 20h9"), p("M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z")],
  external: [p("M15 3h6v6"), p("M10 14 21 3"), p("M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6")],
  bell: [p("M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"), p("M10.3 21a1.94 1.94 0 0 0 3.4 0")],
  lock: [r(4, 11, 16, 10), p("M8 11V7a4 4 0 0 1 8 0v4")],
  keyboard: [r(2, 5, 20, 14), p("M6 9h.01M10 9h.01M14 9h.01M18 9h.01M7 15h10")],
  archive: [r(2, 3, 20, 5, 1), p("M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"), p("M10 12h4")],
  play: p("M7 4v16l13-8Z"),
  stop: r(6, 6, 12, 12, 2),
  power: [p("M12 2v10"), p("M18.4 6.6a9 9 0 1 1-12.77.04")],
  pause: [p("M10 15V9"), p("M14 15V9"), c(12, 12, 10)],
  clock: [c(12, 12, 10), p("M12 6v6l4 2")],
  alert: [p("m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"), p("M12 9v4"), p("M12 17h.01")],
  info: [c(12, 12, 10), p("M12 16v-4"), p("M12 8h.01")],
  question: [c(12, 12, 10), p("M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"), p("M12 17h.01")],
  "check-circle": [c(12, 12, 10), p("m9 12 2 2 4-4")],
  "x-circle": [c(12, 12, 10), p("m15 9-6 6"), p("m9 9 6 6")],
  circle: c(12, 12, 9),
  "circle-dot": [c(12, 12, 9), c(12, 12, 4, true)],
  "circle-half": [c(12, 12, 9), <path key="h" d="M12 3a9 9 0 0 1 0 18Z" fill="currentColor" />],
  "circle-dashed": <circle cx={12} cy={12} r={9} fill="none" strokeDasharray="3.5 3.5" />,
  spinner: p("M21 12a9 9 0 1 1-6.22-8.56"),
  ban: [c(12, 12, 10), p("m4.9 4.9 14.2 14.2")],
  more: [c(5, 12, 1, true), c(12, 12, 1, true), c(19, 12, 1, true)],
  split: [r(3, 3, 18, 18), p("M12 3v18")],
  unified: [r(3, 3, 18, 18), p("M3 12h18")],
  inbox: [p("M22 12h-6l-2 3h-4l-2-3H2"), p("M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z")],
  // A pin: what's in your focus (shell/useFocus.ts).
  pin: [p("M12 17v5"), p("M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z")],
  flag: [p("M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"), p("M4 22v-7")],
  tool: p("M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"),
  book: [p("M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"), p("M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z")],
  layers: [p("m12 2 10 5-10 5L2 7Z"), p("m2 17 10 5 10-5"), p("m2 12 10 5 10-5")],
  gavel: [p("m14 13-7.5 7.5a2.12 2.12 0 0 1-3-3L11 10"), p("m16 16 6-6"), p("m8 8 6-6"), p("m9 7 8 8"), p("m21 11-8-8")],
  hash: p("M4 9h16M4 15h16M10 3 8 21M16 3l-2 18"),
  // Writing: the document editor's toolbar.
  bold: p("M6 12h9a4 4 0 0 1 0 8H7a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1h7a4 4 0 0 1 0 8"),
  italic: p("M19 4h-9M14 20H5M15 4 9 20"),
  strike: p("M16 4H9a3 3 0 0 0-2.83 4M14 12a4 4 0 0 1 0 8H6M4 12h16"),
  code: p("m16 18 6-6-6-6M8 6l-6 6 6 6"),
  link: [p("M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"), p("M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71")],
  list: p("M3 6h.01M3 12h.01M3 18h.01M8 6h13M8 12h13M8 18h13"),
  "list-ordered": p("M10 6h11M10 12h11M10 18h11M4 6h1v4M4 10h2M6 18H4c0-1 2-2 2-3s-1-1.5-2-1"),
  "list-checks": p("m3 7 2 2 4-4M3 17l2 2 4-4M13 6h8M13 12h8M13 18h8"),
  quote: p("M17 6H3M21 12H8M21 18H8M3 12v6"),
  "code-block": [r(3, 3, 18, 18), p("m10 9-3 3 3 3M14 15l3-3-3-3")],
  // An artifact: a page in a window of its own.
  artifact: [r(2, 4, 20, 16), p("M2 9h20"), p("M6 6.5h.01M9 6.5h.01")],
} satisfies Record<string, ReactNode>;

export type IconName = keyof typeof icons;

export function Icon({ name, size = 14, className, title }: { name: IconName; size?: number; className?: string; title?: string }) {
  const body = icons[name];
  return (
    <svg
      className={`icon icon-${name}${className ? ` ${className}` : ""}`}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden={title ? undefined : true}
      role={title ? "img" : undefined}
    >
      {title && <title>{title}</title>}
      {Array.isArray(body) ? body.map((b, i) => <g key={i}>{b}</g>) : body}
    </svg>
  );
}
