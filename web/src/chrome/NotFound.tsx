import { useTitle } from "../ui/useTitle";
import { WindowNotFound } from "./WindowNotFound";

// NotFound is the page at an address that's nothing, titled so.
export function NotFound() {
  useTitle("Not found");
  return <WindowNotFound />;
}
