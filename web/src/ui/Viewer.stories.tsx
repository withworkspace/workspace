import type { Meta, StoryObj } from "@storybook/react-vite";
import { useEffect } from "react";
import { view, Viewer, type Viewing } from "./Viewer";

// The viewer: a diagram on its well, a picture on a checkerboard (so
// what's see-through in it shows), each with its title and the tools; and
// a picture that didn't load. Drag to move, scroll to zoom.

const meta: Meta = { title: "Components/Viewer", parameters: { layout: "fullscreen" } };
export default meta;

const diagram = `<svg xmlns="http://www.w3.org/2000/svg" width="420" height="160" viewBox="0 0 420 160" font-family="sans-serif" font-size="13"><g fill="none" stroke="currentColor"><rect x="10" y="55" width="100" height="50" rx="8"/><rect x="160" y="55" width="100" height="50" rx="8"/><rect x="310" y="55" width="100" height="50" rx="8"/><path d="M110 80H160M260 80H310"/></g><g fill="currentColor" text-anchor="middle"><text x="60" y="85">Brief</text><text x="210" y="85">Work</text><text x="360" y="85">Verify</text></g></svg>`;

const picture = `data:image/svg+xml,${encodeURIComponent(
  `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="300" viewBox="0 0 480 300"><circle cx="170" cy="150" r="110" fill="#7c5cff" fill-opacity="0.8"/><circle cx="310" cy="150" r="110" fill="#ffb454" fill-opacity="0.8"/></svg>`,
)}`;

// Opened shows one, as view() does from anywhere, and closes it again
// when the story's left.
function Opened({ v }: { v: Viewing }) {
  useEffect(() => {
    view(v);
    return () => view(null);
  }, [v]);
  return (
    <div style={{ height: "100vh" }}>
      <Viewer />
    </div>
  );
}

export const Diagram: StoryObj = { render: () => <Opened v={{ kind: "svg", svg: diagram, title: "How a mission moves" }} /> };

export const Picture: StoryObj = { render: () => <Opened v={{ kind: "image", src: picture, title: "two-circles.svg" }} /> };

export const DidNotLoad: StoryObj = { render: () => <Opened v={{ kind: "image", src: "/nothing-here.png", title: "gone.png" }} /> };
