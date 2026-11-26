import { Button } from "../ui/Button";
import { Copyable } from "../ui/Copyable";
import { Icon } from "../ui/Icon";
import css from "./ArtifactHome.module.css";
import { Text } from "../ui/Text";

type ArtifactHomeProps = {
  // Its folder in the repo, as it's named for other people, the repo, and
  // the version that's there (at which commit).
  where: string;
  repo: string;
  version: number;
  commit?: string;
  // The one press that puts the version on the page there, where that's
  // another: which version, and whether it's landing now.
  put?: { n: number; busy: boolean; onPress: () => void };
};

// ArtifactHome says where a promoted artifact lives in a repo, and which
// version is there, read as a sentence, with the press that puts the
// version on the page there after it.
export function ArtifactHome({ where, repo, version, commit, put }: ArtifactHomeProps) {
  return (
    <Text as="div" dim className={css.home}>
      <Icon name="branch" size={12} /> Lives in{" "}
      <Copyable text={where} className={css.where} title="Copy folder path" /> too, for everyone with {repo}:
      v{version} is there{commit ? ` (${commit.slice(0, 7)})` : ""}.
      {put ? (
        <Button variant="ghost" size="small" className={css.action} onClick={put.onPress} disabled={put.busy} title={`Replace v${version} with v${put.n}, in one commit`}>
          <Icon name="check-circle" size={12} /> {put.busy ? "Landing…" : `Replace with v${put.n}`}
        </Button>
      ) : null}
    </Text>
  );
}
