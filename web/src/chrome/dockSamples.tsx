import type { ReactNode } from "react";
import { CrewBadge } from "../mission/parts";
import { Icon } from "../ui/Icon";
import { Status } from "../ui/status";
import { Turn, TurnSaid } from "../ui/Turn";

// Things to peek at, for the docks' stories: Sorrel's, written by hand,
// since the panes that will draw real ones aren't parts yet. What a pane
// shows here is words standing in for it.

export type SampleThing = {
  id: string;
  // What its tab says, and leads with.
  name: string;
  lead: ReactNode;
  // Under its tab: how it stands, where it is.
  sub?: ReactNode;
  // What its pane shows.
  body: ReactNode;
  // It can be said something to: what its box asks for.
  says?: string;
};

const doc = (children: ReactNode) => <div style={{ display: "grid", gap: 10 }}>{children}</div>;
const h = (text: string) => <h4 style={{ margin: "8px 0 0", fontSize: "var(--text-md)" }}>{text}</h4>;
const p = (text: string) => <p style={{ margin: 0 }}>{text}</p>;

// A conversation: what was asked, and what its agent said.
export const rounding: SampleThing = {
  id: "rounding",
  name: "Where do we round money, and is it the same everywhere?",
  lead: <Status s={{ icon: "reply", tone: "attention", words: "Your turn" }} iconOnly size={13} />,
  sub: "Your turn",
  says: "Ask a follow-up…",
  body: (
    <div style={{ display: "grid", gap: 14 }}>
      <Turn by="you" dense landed>
        <TurnSaid written>Where do we round money, and is it the same everywhere?</TurnSaid>
      </Turn>
      <Turn by="agent" dense landed>
        <TurnSaid className="ask-answer">
          {doc(
            <>
              {p("In one place, by one rule.")}
              {p("api/internal/invoice/invoice.go has roundDiv, which rounds half away from zero. Nothing else in the api divides an amount.")}
              {p("The dashboard never rounds: it's given minor units and divides by 100 only to show them.")}
              {p("ADR 0001 says a rounding is a decision, written down where it's made. It doesn't say which way we round, so that's worth a line there once the fix lands.")}
            </>,
          )}
        </TurnSaid>
      </Turn>
    </div>
  ),
};

// A document: a proposal, as it reads.
export const billing: SampleThing = {
  id: "billing",
  name: "Usage-based billing",
  lead: <Icon name="file" size={13} />,
  sub: "Proposal, on the default branch · records",
  body: doc(
    <>
      {h("Problem")}
      {p("Studios that bill by the hour build each invoice by hand from a timesheet. The hours are already somewhere else.")}
      {h("Proposal")}
      {p("A line can take its quantity from a meter: hours from a time tracker, seats from an identity provider. The invoice is drafted at the end of the period with what the meter says, and nothing is sent until someone has read it.")}
      {h("Open questions")}
      {p("Which meters first. Time tracking is most of the asks.")}
      {p("What a meter that's down does to a draft.")}
    </>,
  ),
};

export const freezes: SampleThing = {
  id: "freezes",
  name: "ADR 0003: An invoice freezes when it is sent",
  lead: <Icon name="file" size={13} />,
  sub: "Decision, accepted · records",
  body: doc(
    <>
      {h("Decision")}
      {p("Once an invoice is sent, its lines, rates and totals don't change. A correction is a credit note and a new invoice.")}
      {h("Why")}
      {p("A customer's copy and ours have to agree, and a total that moves after it's been paid can't be explained.")}
    </>,
  ),
};

export const crew: SampleThing = {
  id: "crew",
  name: "Dark mode for the dashboard",
  lead: <CrewBadge id="c1" live />,
  sub: "claude on opus, in web/dark-mode",
  says: "Tell c1 something: it reads it when this step's done",
  body: doc(
    <>
      {p("Writing the dark theme: the variables in theme.css, under prefers-color-scheme: dark.")}
      <p className="dim" style={{ margin: 0 }}>
        Edit web/src/theme.css
      </p>
    </>,
  ),
};

// A page's own tab: something of the page's, beside it.
export const details: SampleThing = {
  id: "details",
  name: "Details",
  lead: <Icon name="list" size={13} />,
  body: doc(
    <>
      {p("Active, at step 4 of 5: Read the fix.")}
      {p("Plan: Reproduce it, Is this the cause?, Fix it, Read the fix, Open it.")}
    </>,
  ),
};

const missions = ["Paginate the invoices endpoint", "Reminders for overdue invoices", "Invoice totals off by a penny", "Export invoices as CSV", "Rate-limit the public API", "Retry failed webhooks with backoff", "Rotate the webhook signing key", "Show VAT by rate on the PDF"];

// More than fit in a row.
export const many: SampleThing[] = [
  rounding,
  billing,
  freezes,
  crew,
  ...missions.map((name, i): SampleThing => ({ id: `m${i}`, name, lead: <CrewBadge id={`c${(i % 3) + 1}`} />, sub: "claude, at work", body: p(`Crew at work on ${name}.`) })),
];
