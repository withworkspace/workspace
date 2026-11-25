import { useParams } from "react-router";
import { PluginFrame } from "./PluginFrame";

// A plugin's page is its own frame and its own connection: another
// plugin's is another of each.
export function PluginPage() {
  const { name = "" } = useParams();
  return <PluginFrame key={name} name={name} />;
}
