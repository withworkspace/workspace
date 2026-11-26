import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "./Button";
import { confirm, Confirmer } from "./Confirm";
import { Dialog, DialogActions, DialogText } from "./Dialog";
import { Option, Select } from "./Select";
import { Input } from "./Input";
import { TextArea } from "./TextArea";
import { Text } from "./Text";

// Dialogs: press a button to open one. As it is, wide with fields (the
// focus starts in the first), bare, and what confirm asks: one that does
// it on Enter, and a danger, which doesn't.

const meta: Meta = { title: "Components/Dialog" };
export default meta;

function Opens({ label, children }: { label: string; children: (close: () => void) => React.ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button onClick={() => setOpen(true)}>{label}</Button>
      {open && children(() => setOpen(false))}
    </>
  );
}

function Fields({ close }: { close: () => void }) {
  const [kind, setKind] = useState("adr");
  return (
    <Dialog title="File as a document" onClose={close} wide>
      <DialogText>It's written into the repo's records, and opens there.</DialogText>
      <div style={{ display: "grid", gap: 10, marginBottom: 16 }}>
        <Select fill value={kind} onChange={setKind} aria-label="Kind">
          <Option value="adr">ADR</Option>
          <Option value="note">Note</Option>
          <Option value="proposal">Proposal</Option>
        </Select>
        <Input type="text" placeholder="What it's named from" aria-label="Title" />
        <TextArea rows={3} placeholder="Why" aria-label="Why" />
      </div>
      <DialogActions>
        <Button onClick={close}>Cancel</Button>
        <Button variant="primary" onClick={close}>
          File it
        </Button>
      </DialogActions>
    </Dialog>
  );
}

export const Kinds: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
      <Opens label="A dialog">
        {(close) => (
          <Dialog title="Remove sketch?" onClose={close}>
            <DialogText>Its sketches stay in their repos. It can be added again.</DialogText>
            <DialogActions>
              <Button onClick={close}>Cancel</Button>
              <Button variant="danger" onClick={close}>
                Remove
              </Button>
            </DialogActions>
          </Dialog>
        )}
      </Opens>
      <Opens label="Wide, with fields">{(close) => <Fields close={close} />}</Opens>
      <Opens label="Bare">
        {(close) => (
          <Dialog title="What's new" onClose={close} bare className="story-bare">
            <div style={{ padding: "8px 28px 8px 0" }}>
              <h2>A head of its own</h2>
              <Text as="p" dim>The close button sits over its corner.</Text>
            </div>
          </Dialog>
        )}
      </Opens>
      <Button onClick={() => void confirm({ title: "Park the mission?", body: "Its crew stop. You can take it up again.", confirm: "Park it" })}>confirm()</Button>
      <Button onClick={() => void confirm({ title: "Burn the capsule?", body: "Its folder and branch are removed.", confirm: "Burn it", danger: true })}>confirm(), a danger</Button>
      <Confirmer />
      <style>{".story-bare { position: relative; }"}</style>
    </div>
  ),
};
