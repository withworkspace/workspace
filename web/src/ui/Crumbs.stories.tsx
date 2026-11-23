import type { Meta, StoryObj } from "@storybook/react-vite";
import { Link } from "react-router";
import { Crumbs, CrumbSep } from "./Crumbs";
import { PageHead } from "./PageHead";

// Crumbs: the way back over a page's head, two steps (where it's in, and
// where else it's reached from), and a path made of the page's own
// pieces, with a slash between.

const meta: Meta = { title: "Components/Crumbs" };
export default meta;

export const Rows: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 24 }}>
      <div>
        <Crumbs steps={[{ to: "/missions/export-invoices-as-csv", label: "Export invoices as CSV" }]} />
        <PageHead title="api-1" />
      </div>
      <div>
        <Crumbs
          steps={[
            { to: "/missions/export-invoices-as-csv", label: "Export invoices as CSV" },
            { to: "/orchestrator", label: "All runs" },
          ]}
        />
        <PageHead title="Orchestrator run" />
      </div>
      <div>
        <Crumbs>
          <Link to="/documents">Documents</Link>
          <CrumbSep>/</CrumbSep>
          <Link to="/documents/records">records</Link>
          <CrumbSep>/</CrumbSep>
          <span aria-current="page">Usage-based billing</span>
        </Crumbs>
      </div>
    </div>
  ),
};
