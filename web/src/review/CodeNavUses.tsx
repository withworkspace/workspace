import { Button } from "../ui/Button";
import { CodeNavList, codeNavListWhereClass } from "./CodeNavList";
import { CodeNavSig } from "./CodeNavSig";
import css from "./CodeNavUses.module.css";
import { Text } from "../ui/Text";

// CodeNavUse is one place a name's used: where, whether the change
// touches that line, and the line.
export interface CodeNavUse {
  key: string;
  path: string;
  line: number;
  text: string;
  changed: boolean;
  picked: boolean;
  onEnter: () => void;
  onGo: () => void;
}

// CodeNavUses is who uses a definition: the definition (def), then its
// uses in groups (in this change first, then elsewhere), each with the
// maybes folded under it (open shows them).
export function CodeNavUses({
  def,
  groups,
  onMaybes,
}: {
  def?: { signature: string; path: string; line: number };
  groups: { title: string; named: boolean; uses: CodeNavUse[]; maybes: CodeNavUse[]; open: boolean }[];
  onMaybes: (title: string) => void;
}) {
  const rows = (us: CodeNavUse[]) =>
    us.map((u) => ({
      key: u.key,
      picked: u.picked,
      onEnter: u.onEnter,
      onClick: u.onGo,
      children: (
        <>
          <Text mono dim className={codeNavListWhereClass}>
            {u.path}:{u.line}
            {u.changed && <span className={css.changed}> changed</span>}
          </Text>
          <CodeNavSig>{u.text}</CodeNavSig>
        </>
      ),
    }));
  return (
    <>
      {def && (
        <div className={css.def}>
          <CodeNavSig>{def.signature}</CodeNavSig>
          <Text mono dim>
            {def.path}:{def.line}
          </Text>
        </div>
      )}
      {!groups.length && <Text as="div" dim className={css.none}>Nothing else uses it.</Text>}
      {groups.map((g) => (
        <section key={g.title}>
          {g.named && <h3 className={css.title}>{g.title}</h3>}
          <CodeNavList rows={rows(g.uses)} />
          {g.maybes.length > 0 && (
            <>
              <Button variant="ghost" size="small" className={css.maybes} onClick={() => onMaybes(g.title)} title="Uses that weren't followed to it, but could be it: one of several by that name, or an interface's method it may satisfy">
                {g.open ? "Hide" : "Show"} {g.maybes.length} that may be
              </Button>
              {g.open && <CodeNavList maybes rows={rows(g.maybes)} />}
            </>
          )}
        </section>
      ))}
    </>
  );
}
