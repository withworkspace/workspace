import type { Meta, StoryObj } from "@storybook/react-vite";
import { SiloPointAsk } from "./SiloPointAsk";

// Pointing a repo's silo at a capsule, as ws's router asks it: the
// question, while it's being pointed, when it couldn't be, and the page
// opened with no capsule to point it at.

const meta: Meta = { title: "Silo/SiloPointAsk" };
export default meta;

const none = () => {};

export const Asked: StoryObj = { render: () => <SiloPointAsk repo="demo" to="rate-limit" then onPoint={none} onNotNow={none} /> };

export const Pointing: StoryObj = { render: () => <SiloPointAsk repo="demo" to="rate-limit" then busy onPoint={none} onNotNow={none} /> };

export const Couldnt: StoryObj = { render: () => <SiloPointAsk repo="demo" to="rate-limit" error="The silo has changes of its own: commit or drop them first." onPoint={none} onNotNow={none} /> };

export const NoCapsule: StoryObj = { render: () => <SiloPointAsk repo="demo" to="" onPoint={none} onNotNow={none} /> };
