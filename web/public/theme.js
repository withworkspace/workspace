// Sets the theme before the page draws, so it never flashes the wrong one:
// your choice in this browser (ws-theme: light or dark), else the system's,
// and how deep the dark theme goes (ws-depth, 1 to 100, as --dark-depth).
// In a frame (a plugin's app, in ws's page) it also marks <html> framed, so
// the shell can leave the page see-through and ws's backdrop shows behind it.
(function () {
  try {
    if (window.self !== window.top) document.documentElement.setAttribute("data-framed", "");
  } catch (e) {}
  var choice;
  try {
    choice = localStorage.getItem("ws-theme");
  } catch (e) {}
  var dark = choice === "dark" || (choice !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  var depth;
  try {
    depth = Math.round(Number(localStorage.getItem("ws-depth")));
  } catch (e) {}
  if (depth > 0 && depth <= 100) document.documentElement.style.setProperty("--dark-depth", String(depth / 100));
})();
