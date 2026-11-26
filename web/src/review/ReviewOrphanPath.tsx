import css from "./ReviewOrphanPath.module.css";
import { Text } from "../ui/Text";

// ReviewOrphanPath is the file a comment the diff doesn't have is on,
// over it, or that it's on the change as a whole.
export function ReviewOrphanPath({ path }: { path?: string }) {
  return <Text as="div" mono dim className={css.path}>{path || "On the change as a whole: it goes in the review's body"}</Text>;
}
