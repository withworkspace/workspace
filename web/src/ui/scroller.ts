// pageScroller is what scrolls a page: App's <main>, as the shell around
// it stays put. Code that follows or moves the page's scroll asks it.
export const pageScroller = () => document.getElementById("main");
