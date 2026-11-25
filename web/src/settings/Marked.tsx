import { usePage } from "./page";
import { SettingsMarked } from "./SettingsMarked";

// Marked is text with what Find has typed marked in it.
export function Marked({ text }: { text: string }) {
  const { marked } = usePage();
  return <SettingsMarked text={text} words={marked} />;
}
