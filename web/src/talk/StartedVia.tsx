import { Link } from "react-router";
import { Text } from "../ui/Text";

// StartedVia says what started a conversation and wrote its first
// message: one of your automations (linked to its page), or a plugin.
export function StartedVia({ via }: { via: string }) {
  const automation = via.startsWith("automation:") ? via.slice("automation:".length) : "";
  if (automation) {
    return (
      <Text as="p" dim>
        Started by your automation <Link to={`/automations/${encodeURIComponent(automation)}`}>{automation}</Link>, which wrote the first message.
      </Text>
    );
  }
  return (
    <Text as="p" dim>
      Started from the {via} plugin, which wrote the first message.
    </Text>
  );
}
