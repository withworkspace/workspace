import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { Option, Select } from "../ui/Select";
import css from "./SettingsAddRepoBar.module.css";
import { Text } from "../ui/Text";

const ANOTHER = "\u0000another";

type SettingsAddRepoBarProps = {
  // Whose repos there are to pick from: you, and the orgs you're in.
  owners: { login: string; you?: boolean }[];
  // The one whose repos are listed, or another, named.
  owner: string;
  another: boolean;
  // The field for another's name is there, and what's typed in it.
  naming: boolean;
  named: string;
  filter: string;
  // When GitHub was last asked, in words, or that it's being asked now.
  when: string;
  looking?: boolean;
  onOwner: (login: string) => void;
  onAnother: () => void;
  onNamed: (text: string) => void;
  onFilter: (text: string) => void;
  onLook: () => void;
};

// SettingsAddRepoBar is the top of Add a repo: whose repos, a filter over
// them, and how old GitHub's answer is, with the way to ask again.
export function SettingsAddRepoBar({ owners, owner, another, naming, named, filter, when, looking, onOwner, onAnother, onNamed, onFilter, onLook }: SettingsAddRepoBarProps) {
  return (
    <div className={css.top}>
      {owners.length > 0 ? (
        <Select aria-label="Whose repos" value={another ? ANOTHER : owner} onChange={(v) => (v === ANOTHER ? onAnother() : onOwner(v))}>
          {[
            ...owners.map((o) => (
              <Option key={o.login} value={o.login} note={o.you ? "you" : undefined}>
                {o.login}
              </Option>
            )),
            <Option key={ANOTHER} value={ANOTHER}>
              Another owner…
            </Option>,
          ]}
        </Select>
      ) : null}
      {naming && <Input className={css.owner} placeholder="owner on GitHub" aria-label="Owner on GitHub" value={named} onChange={(e) => onNamed(e.target.value)} autoFocus />}
      <Input className={css.filter} placeholder="Filter…" aria-label="Filter the repos" value={filter} onChange={(e) => onFilter(e.target.value)} />
      <Text dim className={css.when}>
        {when}
        {!looking && (
          <Button variant="link" onClick={onLook}>
            Look again
          </Button>
        )}
      </Text>
    </div>
  );
}
