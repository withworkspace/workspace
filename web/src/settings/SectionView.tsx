import type { SettingsInfo } from "../api";
import { PluginsSection } from "../plugins/Settings/PluginsSection";
import { PageHead } from "../ui/PageHead";
import { Skeleton } from "../ui/Skeleton";
import { useWorkspace } from "../workspace/store";
import { About } from "./About";
import { BlockView } from "./BlockView";
import { ProcessAbout } from "./ProcessAbout";
import { ProcessFile } from "./ProcessFile";
import { Repos } from "./Repos";
import type { Section } from "./sections";
import { Yours } from "./Yours";

// SectionView is the section you're in: its name, what it's about, its
// blocks, and what's the section's own beside them (the plugins, the
// repos, the process as it's written).
export function SectionView({ section, sections, info, loading }: { section: Section; sections: Section[]; info?: SettingsInfo; loading: boolean }) {
  const branch = useWorkspace((w) => w.defaultBranch);
  const org = useWorkspace((w) => w.org);
  // The sections whose rows are all the page's own needn't wait.
  const waits = section.id !== "keyboard" && section.id !== "notifications" && section.id !== "process" && section.id !== "about" && section.id !== "plugins";
  return (
    <>
      <PageHead title={section.title} />
      <p className="lede">{section.id === "process" ? <ProcessAbout /> : section.about}</p>
      {section.id === "yours" && info && <Yours info={info} sections={sections} />}
      {section.id === "about" ? <About info={info} section={section} /> : section.blocks.map((b) => <BlockView key={b.id} block={b} />)}
      {loading && waits && <Skeleton rows={4} />}
      {section.id === "plugins" && <PluginsSection />}
      {section.id === "repos" && <Repos branch={branch} org={org} />}
      {section.id === "process" && <ProcessFile />}
    </>
  );
}
