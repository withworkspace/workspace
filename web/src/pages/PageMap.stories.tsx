import type { Meta, StoryObj } from "@storybook/react-vite";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { PageMap } from "./PageMap";
import { PageMapEmpty } from "./PageMapEmpty";
import { PageMapItem } from "./PageMapItem";
import { PageMapSection } from "./PageMapSection";
import { PageMapSum } from "./PageMapSum";

// A page's map, beside it: what's open and out of date over it, then its
// outline, its cells as each stands, its passes, what's open (unticked),
// and the files it rests on (one changed since); and the map of a page
// with nothing in it yet.

const meta: Meta = { title: "Pages/PageMap" };
export default meta;

const noop = () => {};

export const Full: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gridTemplateColumns: "1fr 236px", gap: 32, height: 820 }}>
      <div />
      <PageMap>
        <PageMapSum>
          <Chip tone="attention">2 open</Chip>
          <Chip tone="attention">1 out of date</Chip>
          <Chip tone="danger">1 not filled</Chip>
          <Chip tone="progress">
            <Spinner size={11} /> 1 filling
          </Chip>
        </PageMapSum>
        <PageMapSection title="Outline">
          {[
            ["Rate limits for the public API", 1],
            ["Goal", 2],
            ["Routes", 2],
            ["Limits", 2],
            ["Per plan", 3],
          ].map(([text, level]) => (
            <li key={text} style={{ paddingLeft: ((level as number) - 1) * 10 }}>
              <PageMapItem onClick={noop}>{text}</PageMapItem>
            </li>
          ))}
        </PageMapSection>
        <PageMapSection title="Cells">
          <li>
            <PageMapItem look="filled" onClick={noop}>
              <Icon name="check" size={11} />
              <span>a table of the routes and who calls each</span>
            </PageMapItem>
          </li>
          <li>
            <PageMapItem look="stale" onClick={noop}>
              <Icon name="alert" size={11} />
              <span>what each plan's limit should be, from the pricing notes</span>
            </PageMapItem>
          </li>
          <li>
            <PageMapItem look="failed" onClick={noop}>
              <Icon name="alert" size={11} />
              <span>the dashboard's retry code</span>
            </PageMapItem>
          </li>
          <li>
            <PageMapItem look="filling" onClick={noop}>
              <Spinner size={11} />
              <span>a sequence diagram of a throttled request</span>
            </PageMapItem>
          </li>
        </PageMapSection>
        <PageMapSection title="Passes">
          <li>
            <PageMapItem look="stale" onClick={noop}>
              <Icon name="alert" size={11} />
              <span>Check: 3 open, the page has changed since</span>
            </PageMapItem>
          </li>
          <li>
            <PageMapItem look="filled" onClick={noop}>
              <Icon name="check" size={11} />
              <span>Formalise: 2 open</span>
            </PageMapItem>
          </li>
        </PageMapSection>
        <PageMapSection title="Open">
          <li>
            <PageMapItem look="open" onClick={noop}>
              Ask support which studios sync hourly
            </PageMapItem>
          </li>
          <li>
            <PageMapItem look="open" onClick={noop}>
              Decide whether the dashboard is exempt
            </PageMapItem>
          </li>
        </PageMapSection>
        <PageMapSection title="Rests on">
          <li>
            <PageMapItem to="/files/api?path=cmd/api/main.go">
              <Icon name="file" size={11} />
              <span>api/cmd/api/main.go</span>
              <span className="dim">×2</span>
            </PageMapItem>
          </li>
          <li>
            <PageMapItem to="/files/records?path=proposals/usage-based-billing.md" look="moved">
              <Icon name="alert" size={11} />
              <span>records/proposals/usage-based-billing.md</span>
            </PageMapItem>
          </li>
          <li>
            <PageMapItem look="moved">
              <Icon name="alert" size={11} />
              <span>api/internal/ratelimit/old.go</span>
            </PageMapItem>
          </li>
        </PageMapSection>
      </PageMap>
    </div>
  ),
};

export const Empty: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gridTemplateColumns: "1fr 236px", gap: 32 }}>
      <div />
      <PageMap>
        <PageMapEmpty />
      </PageMap>
    </div>
  ),
};
