// Sets the theme before the page draws, so it never flashes the wrong one:
// your choice in this browser (ws-theme: light or dark), else the system's.
(function () {
  var choice;
  try {
    choice = localStorage.getItem("ws-theme");
  } catch (e) {}
  var dark = choice === "dark" || (choice !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
})();
