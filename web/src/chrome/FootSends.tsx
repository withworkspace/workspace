import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { FootItem } from "./FootItem";
import css from "./FootSends.module.css";

type Props = {
  // How many things said from pull requests' pages are on their way to
  // GitHub, and how many it didn't take.
  sending?: number;
  failed?: number;
  // Where the first it didn't take is, to go to.
  to?: string;
};

// FootSends is what's been said here and sent to GitHub behind: how many
// are on their way, quietly, and how many it didn't take, which is
// something to do: a press goes to the pull request that says which.
// Nothing's said while there's neither.
export function FootSends({ sending = 0, failed = 0, to = "/reviews" }: Props) {
  if (failed > 0) {
    const words = `${failed} didn’t reach GitHub`;
    return (
      <FootItem to={to} className={css.failed} title={`${words}: go to where it says which, to send again or let go of`}>
        <Icon name="alert" size={13} />
        <span>{words}</span>
      </FootItem>
    );
  }
  if (sending <= 0) return null;
  return (
    <FootItem to={to} title="What you've said on pull requests is on its way to GitHub">
      <Spinner size={11} />
      <span>Sending {sending} to GitHub</span>
    </FootItem>
  );
}
