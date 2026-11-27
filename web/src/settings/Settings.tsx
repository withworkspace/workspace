import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import { Navigate, useLocation, useParams } from "react-router";
import { getSettings, setSettings, type SettingChange, type SettingsInfo } from "../api";
import { useCommands } from "../keymap/useCommands";
import { settingsKeys } from "./keys";
import { useLoaded } from "../load";
import { ErrorCard } from "../ui/ErrorCard";
import { Skeleton } from "../ui/Skeleton";
import { toast } from "../ui/toast";
import { useTitle } from "../ui/useTitle";
import { fileName, written } from "./file";
import { forgetModels } from "../models/models";
import { type Page, PageContext } from "./page";
import { Results } from "./Results";
import { find, minesOf, sectionAt, sectionsOf, wordsOf } from "./sections";
import { SectionView } from "./SectionView";
import { SettingsFind } from "./SettingsFind";
import { SettingsLayout } from "./SettingsLayout";
import { SettingsNav, SettingsNavHeading } from "./SettingsNav";
import { SettingsNavLink } from "./SettingsNavLink";
import { SettingsSide } from "./SettingsSide";

// The Settings page: your settings for this workspace, in sections, the
// list of them beside the sidebar and the one you're in beside that, to a
// width. Most are the ones in ws.local.toml: each says what it is, and
// where its value comes from when that isn't ws's default (the team's
// ws.toml, or yours). A change is written into ws.local.toml at once, key
// by key, so the rest of the file stays as you wrote it; Reset takes yours
// away, and the team's or the default applies again. A few are kept in
// this browser (the theme, desktop notifications), and say so. Each
// section has an address (/settings/agents), and Find reads them all.
//
// This is what the page knows and does: the settings as ws answered for
// them, what's typed into Find, and the writes. What it draws is parts
// that are given what to draw (Settings…, each with its module), by way
// of the components that say which part a thing of the settings' is
// drawn as (SectionView, BlockView, RowOf and the rows, Control and the
// controls).
export function SettingsPage({
  notify,
}: {
  notify: Page["notify"];
}) {
  const { section: at } = useParams();
  const anchor = useLocation().hash.slice(1);
  const { value: info, error, set: setInfo, reload: load } = useLoaded<SettingsInfo>(getSettings, [], { keep: true });
  const [query, setQuery] = useState("");
  const field = useRef<HTMLInputElement>(null);
  const sections = useMemo(() => sectionsOf(info), [info]);
  const section = sections.find((s) => s.id === at);
  useTitle(section ? `${section.title} · Settings` : "Settings");
  useCommands(settingsKeys, { find: () => field.current?.focus() });
  // What a harness lists is asked afresh each time you come here.
  useEffect(forgetModels, []);

  // An address with a block named (/settings/missions#orchestrator) shows it.
  const shownAt = section?.id;
  const loaded = !!info;
  useEffect(() => {
    if (anchor && shownAt && loaded) document.getElementById(anchor)?.scrollIntoView({ block: "start" });
  }, [anchor, shownAt, loaded]);

  // change writes changes together; undo is what puts yours back as it was.
  // Writes are made in turn, each answered before the next is sent, so the
  // page is never shown an older answer after a newer one, and no control
  // need wait (or lose its focus) while another's written.
  const writes = useRef(Promise.resolve());
  const file = info ? fileName(info.file) : "ws.local.toml";
  const change = (changes: SettingChange[], undo?: SettingChange[]) => {
    const write = async () => {
      try {
        setInfo(await setSettings(changes));
        toast(`${changes.map(written).join(", ")} in ${file}`, {
          tone: "done",
          action: undo && { label: "Undo", run: () => void change(undo) },
        });
      } catch (e) {
        toast((e as Error).message, { tone: "danger", ms: 8000 });
        await load();
      }
    };
    writes.current = writes.current.then(write);
    return writes.current;
  };

  // /settings is its first section; an address from when the page was one
  // (/settings#orchestrator) is the section that's in now, once the
  // settings say which; and a section there isn't is the first.
  const settled = loaded || !!error;
  if (!at && !anchor) return <Navigate to={`/settings/${sections[0].id}`} replace />;
  if (!at && settled) return <Navigate to={`/settings/${(sectionAt(sections, anchor) ?? sections[0]).id}#${anchor}`} replace />;
  if (at && !section && settled) return <Navigate to={`/settings/${sections[0].id}`} replace />;

  const finding = query.trim() !== "";
  const found = finding ? find(sections, query) : [];
  const all = info?.groups.flatMap((g) => g.settings) ?? [];
  const page: Page = { all, change, marked: finding ? wordsOf(query) : [], notify };
  const mine = minesOf(sections).length;
  const unread = info?.unknown?.length ?? 0;
  const clear = () => setQuery("");

  return (
    <PageContext.Provider value={page}>
      <SettingsLayout
        side={
          <SettingsSide title="Settings" find={<SettingsFind ref={field} value={query} onChange={setQuery} onClear={clear} />}>
            <SettingsNav>
              {sections.map((sec) => {
                const hits = found.find((f) => f.section.id === sec.id)?.rows.length ?? 0;
                // Yours says how many your file sets, and whether it has keys ws doesn't read.
                const yours = !finding && sec.id === "yours";
                return (
                  <Fragment key={sec.id}>
                    {sec.heading && <SettingsNavHeading>{sec.heading}</SettingsNavHeading>}
                    <SettingsNavLink
                      to={`/settings/${sec.id}`}
                      icon={sec.icon}
                      current={!finding && sec.id === section?.id}
                      faded={finding && hits === 0}
                      count={finding ? (hits > 0 ? hits : undefined) : yours && mine > 0 ? mine : undefined}
                      countTitle={yours && mine > 0 ? `${mine} set by you` : undefined}
                      unread={yours && unread > 0 ? (unread === 1 ? "1 unknown key" : `${unread} unknown keys`) : undefined}
                      onClick={clear}
                    >
                      {sec.title}
                    </SettingsNavLink>
                  </Fragment>
                );
              })}
            </SettingsNav>
          </SettingsSide>
        }
      >
        {error && <ErrorCard error={error} retry={load} />}
        {/* An old address, till the settings say which section it's in now. */}
        {!section && !finding && !settled && <Skeleton rows={6} />}
        {finding ? <Results found={found} query={query} clear={clear} /> : section && <SectionView section={section} sections={sections} info={info} loading={!settled} />}
      </SettingsLayout>
    </PageContext.Provider>
  );
}
