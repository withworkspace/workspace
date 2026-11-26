import { Link } from "react-router";
import { DockBody } from "../chrome/DockBody";
import { useReviewComments } from "../review/comments";
import { ReviewCommentList } from "../review/ReviewCommentList";
import { Text } from "../ui/Text";
import css from "./CommentsPane.module.css";

// CommentsPane is a review's comments beside it: every one, each file's
// together, to read down and go to. The list is the review's, said while
// its page is open (review/comments.ts); with the page gone, it says
// where the review is.
export function CommentsPane({ of }: { of: string }) {
  const comments = useReviewComments();
  if (comments.of !== of)
    return (
      <DockBody label="Comments" pad>
        <Text as="p" dim className={css.away}>
          Its comments are listed while <Link to={of}>the review</Link> is open.
        </Text>
      </DockBody>
    );
  return (
    <DockBody label="Comments">
      <ReviewCommentList list={comments.list} filter={comments.filter} onFilter={comments.setFilter} at={comments.at} onPick={(key) => comments.go?.(key)} />
    </DockBody>
  );
}
