import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Empty } from "./Empty";
import { MoreLink } from "./MoreLink";
import { Text } from "./Text";

// Empty places: a title alone, with a sentence, with something to press;
// then the big one, plain and done, with words after what's pressed.

const meta: Meta = { title: "Components/Empty" };
export default meta;

export const Every: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 8 }}>
      <Empty icon="file" title="An empty file" />
      <Empty icon="box" title="Nothing's been made yet">
        What crew make shows here: capsules, their changes, and documents.
      </Empty>
      <Empty icon="search" title="None here" action={<Button>Show every artifact</Button>}>
        Nothing matches what's typed.
      </Empty>
      <Empty title="The default icon" />
    </div>
  ),
};

export const Big: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 8 }}>
      <Empty
        big
        done
        icon="check"
        title="You're all clear"
        action={<Button variant="primary">Approve b3</Button>}
        foot={
          <>
            <Text as="div" dim>It lands once it's approved.</Text>
            <MoreLink onClick={() => {}}>Look again</MoreLink>
          </>
        }
      >
        Every check left for you is approved, and the two ws ran still pass.
      </Empty>
      <Empty big icon="inbox" title="Nothing waiting" action={<Button>Start a mission</Button>}>
        A big one that isn't of something done: its disc is grey.
      </Empty>
    </div>
  ),
};
