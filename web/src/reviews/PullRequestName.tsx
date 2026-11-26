import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import css from "./PullRequestName.module.css";

// PullRequestName is a pull request's name in its review's bar: its title,
// cut short where there's no room, and where it sits in its stack (layer
// of layers). The whole stack's is the stack's title, and says it's all
// of them. Its repo and number aren't said: the bar's repo tag says the
// one, and the other's on GitHub.
export function PullRequestName({ title, layer, whole }: { title: string; layer?: { n: number; of: number }; whole?: { layers: number } }) {
  return (
    <span className={css.name}>
      <span className={css.title} title={title}>
        {title}
      </span>
      {whole ? (
        <Chip tone="quiet" className={css.chip} title="All layers' changes combined">
          <Icon name="layers" size={11} /> the whole stack · {whole.layers} layers
        </Chip>
      ) : layer ? (
        <Chip tone="quiet" className={css.chip} title="Position in stack, from the bottom">
          <Icon name="layers" size={11} /> {layer.n} of {layer.of}
        </Chip>
      ) : null}
    </span>
  );
}
