import type { Meta, StoryObj } from "@storybook/react-vite";
import { MoreLink } from "../ui/MoreLink";
import { TalkSide } from "./TalkSide";
import { TalkSideEmpty } from "./TalkSideEmpty";
import { TalkSideItem } from "./TalkSideItem";
import { TalkSideMade } from "./TalkSideMade";
import { TalkSideNamed } from "./TalkSideNamed";
import { TalkSideSection } from "./TalkSideSection";
import { TalkSideSummary } from "./TalkSideSummary";

// What's come of a conversation, in the column beside it: what's to
// confirm (each goes to its card), what it made (each goes to the thing,
// with its card a click beside it once pointed at), the capsules it
// lifted, what it names, and its summary so far; and the column before
// anything's come of it.

const meta: Meta = { title: "Talk/TalkSide" };
export default meta;

const none = () => {};

export const Full: StoryObj = {
  render: () => (
    <div style={{ width: 236 }}>
      <TalkSide>
        <TalkSideSection title="To confirm">
          <ul>
            <li>
              <TalkSideItem open icon="message" name="A review comment on api#41" onClick={none} />
            </li>
            <li>
              <TalkSideItem open icon="artifact" name="A version of a1" tag="a1" onClick={none} />
            </li>
          </ul>
        </TalkSideSection>
        <TalkSideSection title="Made here">
          <ul>
            <TalkSideMade onWhere={none}>
              <TalkSideItem grow icon="file" name="Where money is rounded" tag="records:notes/where-money-is-rounded.md" to="/documents" />
            </TalkSideMade>
            <TalkSideMade onWhere={none}>
              <TalkSideItem grow icon="artifact" name="Late fee settings" tag="a2.v1" to="/artifacts/a2" />
            </TalkSideMade>
            <TalkSideMade>
              <TalkSideItem grow icon="gavel" name="Late fees are a flat fee, never interest" onClick={none} />
            </TalkSideMade>
          </ul>
        </TalkSideSection>
        <TalkSideSection title="Capsules" tip="Capsules its agent lifted to try things in">
          <TalkSideNamed>
            <li title="On branch try-redis">
              <code>api/try-redis</code>
            </li>
            <li title="Put away with the conversation">
              <code>web/fee-form</code>
              <span className="dim"> put away</span>
            </li>
          </TalkSideNamed>
        </TalkSideSection>
        <TalkSideSection title="Mentioned">
          <TalkSideNamed refs>
            <li>
              <code>reminders-for-overdue-invoices</code>
            </li>
            <li>
              <code>api#41</code>
            </li>
          </TalkSideNamed>
          <MoreLink onClick={none}>3 more</MoreLink>
        </TalkSideSection>
        <TalkSideSection title="Summary so far">
          <TalkSideSummary text={"Late fees would be **a flat fee**, never interest, sent as an invoice of their own.\n\n- A studio can turn them off.\n- Rounding is once per line."} />
        </TalkSideSection>
      </TalkSide>
    </div>
  ),
};

export const Empty: StoryObj = {
  render: () => (
    <div style={{ width: 236 }}>
      <TalkSide>
        <TalkSideEmpty />
      </TalkSide>
    </div>
  ),
};
