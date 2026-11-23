import type { Meta, StoryObj } from "@storybook/react-vite";
import { Banner } from "./Banner";
import { Button } from "./Button";

// Banners: an error as most are, then in the other tones, with an icon,
// with something to press, and one whose words are a long path.

const meta: Meta = { title: "Components/Banner" };
export default meta;

export const Every: StoryObj = {
  render: () => (
    <div className="page narrow">
      <Banner>The daemon didn't answer.</Banner>
      <Banner tone="attention">It may be behind: the records repo couldn't be fetched.</Banner>
      <Banner tone="progress" icon="box">
        Changed in archive-invoices
      </Banner>
      <Banner tone="done" icon="check" role="status" actions={<Button size="small">Open the document</Button>}>
        Filed as records · docs/invoices.md, landed an hour ago. It's edited there now.
      </Banner>
      <Banner
        tone="attention"
        actions={
          <>
            <Button size="small">Read it again</Button>
            <Button size="small">Keep what's here</Button>
          </>
        }
      >
        This page was changed somewhere else (another tab, perhaps) since you opened it here. Nothing more is saved from here until you choose.
      </Banner>
      <Banner>open /Users/someone/Projects/workspace/repos/records/artifacts/a3/versions/2/assets/invoice-totals-off-by-a-penny.json: no such file or directory</Banner>
    </div>
  ),
};
