// What app.js imports: one release, as its section.
export function release(r) {
  const section = document.createElement("section");
  section.className = "release";
  const h = document.createElement("h2");
  h.textContent = r.version;
  const when = document.createElement("time");
  when.textContent = r.date;
  const list = document.createElement("ul");
  for (const c of r.changes) {
    const li = document.createElement("li");
    const kind = document.createElement("span");
    kind.className = "kind";
    kind.textContent = c.kind;
    li.append(kind, c.text);
    list.append(li);
  }
  section.append(h, when, list);
  return section;
}
