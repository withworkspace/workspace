// briefGoal is the brief's goal in a line: the first paragraph under a Goal
// heading, wherever it is, else the first paragraph that isn't a heading.
export function briefGoal(brief: string): string {
  const lines = brief.split("\n");
  const firstParagraph = (from: number) => {
    const para: string[] = [];
    for (const line of lines.slice(from)) {
      const t = line.trim();
      if ((t.startsWith("#") || t === "") && para.length) break;
      if (t === "" || t.startsWith("#")) continue;
      para.push(t);
    }
    return para.join(" ");
  };
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (t.startsWith("#") && t.replace(/^#+/, "").trim().toLowerCase() === "goal") {
      const goal = firstParagraph(i + 1);
      if (goal) return goal;
    }
  }
  return firstParagraph(0);
}
