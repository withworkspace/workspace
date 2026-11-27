import { type ChartSeries, LineChart } from "../ui/LineChart";
import { CodeBlock } from "./CodeBlock";
import css from "./Markdown.module.css";

// MarkdownChart is a ```chart block drawn: its JSON, as an automation's
// chart() writes it ({"title", "unit", "series": [{"name", "points":
// [[at, value], …]}]}). One that isn't is shown as the code it is.
export function MarkdownChart({ text, data }: { text: string; data: Record<string, unknown> }) {
  const c = parse(text);
  if (!c) return <CodeBlock lang="json" html={escape(text)} className={css.code} data={data} />;
  return (
    <div {...data}>
      <LineChart title={c.title} unit={c.unit} series={c.series} />
    </div>
  );
}

function parse(text: string): { title?: string; unit?: string; series: ChartSeries[] } | undefined {
  try {
    const v = JSON.parse(text) as { title?: unknown; unit?: unknown; series?: unknown };
    if (!Array.isArray(v.series)) return undefined;
    const series = v.series
      .filter((s): s is { name?: unknown; points?: unknown } => typeof s === "object" && s !== null)
      .map((s) => ({
        name: typeof s.name === "string" ? s.name : "",
        points: (Array.isArray(s.points) ? s.points : [])
          .filter((p): p is [string, number] => Array.isArray(p) && typeof p[0] === "string" && typeof p[1] === "number")
          .map((p) => [p[0], p[1]] as [string, number]),
      }));
    return { title: typeof v.title === "string" ? v.title : undefined, unit: typeof v.unit === "string" ? v.unit : undefined, series };
  } catch {
    return undefined;
  }
}

function escape(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
