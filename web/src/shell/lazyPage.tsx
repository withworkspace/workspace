import { type ComponentType, use } from "react";

// Every page made with lazyPage, as what starts loading it.
const pages: (() => void)[] = [];

// lazyPage is a page whose code is a chunk of its own, kept out of the
// script the web UI starts with. The page at the address loads first, on
// being drawn; Pages has the rest load once it's shown (loadPages), so by
// the time you go anywhere else its code has come and the page is drawn
// straight away. Going somewhere before then is a transition (the router
// makes every one), which keeps the page you're on until the next one's
// ready: there's never an empty frame. Loading them all early also means a
// tab left open over an upgrade doesn't ask for a chunk the new build no
// longer has.
export function lazyPage<P extends object>(load: () => Promise<ComponentType<P>>): ComponentType<P> {
  let loaded: ComponentType<P> | undefined;
  let loading: Promise<ComponentType<P>> | undefined;
  const start = () => (loading ??= load().then((page) => (loaded = page)));
  pages.push(start);
  return function LazyPage(props: P) {
    const Page = loaded ?? use(start());
    return <Page {...props} />;
  };
}

// loadPages starts loading every page that isn't already. It's after the
// first page is shown, so their chunks don't hold up its code and the
// workspace's first reads, which wait for the same few connections.
export function loadPages() {
  for (const start of pages) start();
}
