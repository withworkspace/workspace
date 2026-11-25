import type { Ref } from "react";
import css from "./MissionCrewScreen.module.css";

// MissionCrewScreen is where a crew's terminal draws its screen (ref: the
// element xterm opens in), and what wasn't sent to it, under it.
export function MissionCrewScreen({ ref, error }: { ref: Ref<HTMLDivElement>; error?: string }) {
  return (
    <>
      <div ref={ref} className={css.screen} />
      {error && <div className={css.error}>Not sent: {error}</div>}
    </>
  );
}
