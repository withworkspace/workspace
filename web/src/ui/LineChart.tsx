import css from "./LineChart.module.css";

// LineChart is numbers over time, a line each series: its title, the
// value's unit (%), the latest of each said at its end, the first and last
// day under it, and three of its values beside. An automation's output
// draws one from a ```chart block (markdown/MarkdownChart), as charts
// anywhere in markdown are. A series is points, each when (ISO) and how
// much; with more than one, each's named in a key under it.
export interface ChartSeries {
  name: string;
  points: [string, number][];
}

// seriesClass is each series's colour, in order.
const seriesClass = [css.s0, css.s1, css.s2, css.s3, css.s4, css.s5];

const W = 640;
const H = 200;
const PAD = { top: 12, right: 64, bottom: 24, left: 44 };

export function LineChart({ title, unit = "", series }: { title?: string; unit?: string; series: ChartSeries[] }) {
  const all = series.flatMap((s) => s.points.map(([at, v]) => ({ t: Date.parse(at), v }))).filter((p) => !Number.isNaN(p.t));
  if (all.length === 0) {
    return (
      <figure className={css.chart}>
        {title && <figcaption className={css.title}>{title}</figcaption>}
        <div className={css.none}>No points yet: it shows once a run has kept some.</div>
      </figure>
    );
  }
  const t0 = Math.min(...all.map((p) => p.t));
  const t1 = Math.max(...all.map((p) => p.t));
  const [lo, hi] = bounds(Math.min(...all.map((p) => p.v)), Math.max(...all.map((p) => p.v)));
  const x = (t: number) => PAD.left + (t1 === t0 ? (W - PAD.left - PAD.right) / 2 : ((t - t0) / (t1 - t0)) * (W - PAD.left - PAD.right));
  const y = (v: number) => PAD.top + (1 - (v - lo) / (hi - lo)) * (H - PAD.top - PAD.bottom);
  const ticks = [lo, (lo + hi) / 2, hi];
  const said = (v: number) => `${round(v)}${unit}`;
  return (
    <figure className={css.chart}>
      {title && <figcaption className={css.title}>{title}</figcaption>}
      <svg viewBox={`0 0 ${W} ${H}`} className={css.svg} role="img" aria-label={label(title, series, said)}>
        {ticks.map((v) => (
          <g key={v}>
            <line x1={PAD.left} x2={W - PAD.right} y1={y(v)} y2={y(v)} className={css.grid} />
            <text x={PAD.left - 6} y={y(v)} className={css.tick} textAnchor="end" dominantBaseline="middle">
              {said(v)}
            </text>
          </g>
        ))}
        <text x={PAD.left} y={H - 6} className={css.tick}>
          {day(t0)}
        </text>
        {t1 !== t0 && (
          <text x={W - PAD.right} y={H - 6} className={css.tick} textAnchor="end">
            {day(t1)}
          </text>
        )}
        {series.map((s, i) => {
          const ps = s.points.map(([at, v]) => ({ t: Date.parse(at), v })).filter((p) => !Number.isNaN(p.t)).sort((a, b) => a.t - b.t);
          if (ps.length === 0) return null;
          const last = ps[ps.length - 1];
          return (
            <g key={s.name + i} className={seriesClass[i % seriesClass.length]}>
              {ps.length > 1 && <polyline points={ps.map((p) => `${x(p.t)},${y(p.v)}`).join(" ")} className={css.line} />}
              <circle cx={x(last.t)} cy={y(last.v)} r={3.5} className={css.dot} />
              <text x={x(last.t) + 8} y={y(last.v)} className={css.last} dominantBaseline="middle">
                {said(last.v)}
              </text>
            </g>
          );
        })}
      </svg>
      {series.length > 1 && (
        <ul className={css.key}>
          {series.map((s, i) => (
            <li key={s.name + i} className={seriesClass[i % seriesClass.length]}>
              <span className={css.swatch} />
              {s.name}
            </li>
          ))}
        </ul>
      )}
    </figure>
  );
}

// bounds are a little room above and below what's there; a line that's
// flat is given some.
function bounds(lo: number, hi: number): [number, number] {
  if (hi === lo) return [lo - 1, hi + 1];
  const room = (hi - lo) * 0.1;
  return [lo - room, hi + room];
}

function round(v: number): string {
  return Math.abs(v) >= 100 ? String(Math.round(v)) : String(Math.round(v * 10) / 10);
}

function day(t: number): string {
  return new Date(t).toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

// label is what a screen reader's told of it: each series's latest.
function label(title: string | undefined, series: ChartSeries[], said: (v: number) => string): string {
  const latest = series
    .filter((s) => s.points.length > 0)
    .map((s) => `${s.name || "latest"} ${said(s.points[s.points.length - 1][1])}`)
    .join(", ");
  return `${title ? `${title}: ` : ""}a chart over time; ${latest}`;
}
