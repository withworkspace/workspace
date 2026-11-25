import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import css from "./PullRequestName.module.css";

// PullRequestName is a pull request's name in its review's head: its
// title and where it is (at, its repo#number), and where it sits in its stack (layer of layers). The
// whole stack's is the stack's title, and says it's all of them.
export function PullRequestName({ title, at, layer, whole }: { title: string; at?: string; layer?: { n: number; of: number }; whole?: { layers: number } }) {
  return (
    <span className={css.name}>
      {whole ? (
        <>
          {title}
          <Chip tone="quiet" className={css.chip} title="Every layer's changes together: where the stack ends up">
            <Icon name="layers" size={11} /> the whole stack · {whole.layers} layers
          </Chip>
        </>
      ) : (
        <>
          {title} <span className={`dim ${css.ref}`}>{at}</span>
          {layer ? (
            <Chip tone="quiet" className={css.chip} title="Where this layer sits in its stack: 1 is the bottom">
              <Icon name="layers" size={11} /> {layer.n} of {layer.of}
            </Chip>
          ) : null}
        </>
      )}
    </span>
  );
}
