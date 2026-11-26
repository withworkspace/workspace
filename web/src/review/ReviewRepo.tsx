import { useRefs } from "../refs/known";
import { RepoTag } from "../ui/RepoTag";

// ReviewRepo is which repo a review's change is in, in its bar: as the
// workspace labels and colours it, or by its name until that's known.
export function ReviewRepo({ repo }: { repo: string }) {
  const known = useRefs()?.repos.get(repo);
  return <RepoTag label={known?.label || repo} color={known?.color} title={`In ${known?.label || repo}`} />;
}
