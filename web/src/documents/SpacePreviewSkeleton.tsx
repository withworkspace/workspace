import { Skeleton } from "../ui/Skeleton";
import css from "./SpacePreviewSkeleton.module.css";

// SpacePreviewSkeleton is the document picked while its text is read.
export function SpacePreviewSkeleton() {
  return <Skeleton rows={8} className={css.skeleton} />;
}
