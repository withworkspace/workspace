import css from "./SpaceOtherHead.module.css";

// SpaceOtherHead is the heading over a space's documents of no kind, by
// folder, below its kinds' lists: Other.
export function SpaceOtherHead({ id }: { id: string }) {
  return (
    <h2 className={css.head} id={id}>
      Other
    </h2>
  );
}
