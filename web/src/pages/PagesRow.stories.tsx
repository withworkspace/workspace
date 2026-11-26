import { cx } from "../ui/cx";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../ui/Button";
import { Callout } from "../ui/Callout";
import { Chip } from "../ui/Chip";
import { Dialog, DialogActions } from "../ui/Dialog";
import { Input } from "../ui/Input";
import { Keys } from "../ui/Keys";
import { Rows } from "../ui/Rows";
import { Option, Select } from "../ui/Select";
import { PageField, pageFieldCodeClass } from "./PageField";
import { PageFields } from "./PageFields";
import { PageFileWhat } from "./PageFileWhat";
import { PageFileWhere } from "./PageFileWhere";
import { PageKindPick } from "./PageKindPick";
import { PageKindPicks } from "./PageKindPicks";
import { PagesRow } from "./PagesRow";
import { Text, textClass } from "../ui/Text";

// Pages in a list (a kind's, one tied to a file, one filling its cells,
// one with cells out of date, one filed); and the dialogs that start a
// page as a kind of document, file one, and save one to its file.

const meta: Meta = { title: "Pages/PagesRow" };
export default meta;

const noop = () => {};
const updated = new Date(Date.now() - 3 * 3600_000).toISOString();

export const List: StoryObj = {
  render: () => (
    <Rows>
      <PagesRow id="p2" title="Rate limits for the public API" to="/documents/local/p2" cells={6} stale={2} updated={updated} selected onHover={noop} />
      <PagesRow id="p4" title="Keep rate limits in Redis" to="/documents/local/p4" kind={{ title: "ADR", hint: "An architecture decision record" }} updated={updated} selected={false} onHover={noop} />
      <PagesRow id="p3" title="How VAT is shown on an invoice" to="/documents/local/p3" file={{ name: "how-vat-is-shown-on-an-invoice.md", ref: "records:notes/how-vat-is-shown-on-an-invoice.md" }} updated={updated} selected={false} onHover={noop} />
      <PagesRow id="p6" title="Webhook retries" to="/documents/local/p6" cells={3} filling={2} updated={updated} selected={false} onHover={noop} />
      <PagesRow id="p7" title="Dark mode" to="/documents/local/p7" updated={updated} note={<Chip tone="done">Filed</Chip>} selected={false} onHover={noop} />
    </Rows>
  ),
};

export const StartAsAKind: StoryObj = {
  render: () => (
    <Dialog title="Start a page as…" onClose={noop} wide>
      <PageKindPicks>
        <PageKindPick on onClick={noop}>
          ADR
        </PageKindPick>
        <PageKindPick on={false} onClick={noop}>
          Proposal
        </PageKindPick>
        <PageKindPick on={false} onClick={noop}>
          Note
        </PageKindPick>
      </PageKindPicks>
      <PageFields>
        <PageField label="Title">
          <Input value="Keep rate limits in Redis" onChange={noop} aria-label="Title" />
        </PageField>
      </PageFields>
      <PageFileWhat>
        Its agents are told how one's written (<Text mono>docs/guides/adr.md</Text>), and <em>Draft it</em> lays it out that way. Once it's written, <em>File as…</em> files it at <Text mono>records · adr/NNNN-keep-rate-limits-in-redis.md</Text>.
      </PageFileWhat>
      <DialogActions>
        <Button variant="ghost">Cancel</Button>
        <Button variant="primary">
          Start it <Keys k="⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  ),
};

export const FileAs: StoryObj = {
  render: () => (
    <Dialog title="File as a document" onClose={noop} wide>
      <PageFields>
        <PageField label="Kind">
          <Select fill value="" onChange={noop} aria-label="Kind of document">
            <Option value="adr">ADR</Option>
            <Option value="">No kind: in a folder…</Option>
          </Select>
        </PageField>
        <PageField label="Folder">
          <Input className={cx(textClass("mono"), pageFieldCodeClass)} value="records:notes/" onChange={noop} aria-label="The folder it's filed in, as repo:folder/" />
        </PageField>
        <PageField label="Title">
          <Input value="Rate limits for the public API" onChange={noop} aria-label="Title" />
        </PageField>
        <PageField label="Commit">
          <Input className={cx(textClass("mono"), pageFieldCodeClass)} value="Add the note rate-limits-for-the-public-api" onChange={noop} aria-label="The commit's message" />
        </PageField>
      </PageFields>
      <PageFileWhere at="records:notes/rate-limits-for-the-public-api.md" taken>
        <span>: there's one there already. Give it another title.</span>
      </PageFileWhere>
      <Callout tone="attention" icon="alert" title="Filing lands it on main">
        This repo has no pull requests: it's one commit, straight to main.
      </Callout>
      <PageFileWhat>What's filed is the page as it reads: what its cells hold is kept as text, its questions are left out. The page stays, and says where it was filed.</PageFileWhat>
      <DialogActions>
        <Button variant="ghost">Cancel</Button>
        <Button variant="primary" disabled>
          File, and land on main <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  ),
};

export const SaveToFile: StoryObj = {
  render: () => (
    <Dialog title="Save to the repo" onClose={noop} wide>
      <PageFileWhere at="records:adr/0001-money-is-whole-minor-units.md" />
      <PageFields>
        <PageField label="Commit">
          <Input className={cx(textClass("mono"), pageFieldCodeClass)} value="Amend the ADR money-is-whole-minor-units" onChange={noop} aria-label="The commit's message" />
        </PageField>
      </PageFields>
      <PageFileWhat>What's saved is the page as it reads: what its cells hold is kept as text, its questions are left out. The page keeps them.</PageFileWhat>
      <DialogActions>
        <Button variant="ghost">Cancel</Button>
        <Button variant="primary">
          Save, and land on main <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  ),
};
