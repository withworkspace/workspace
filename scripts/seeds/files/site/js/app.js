// The page's own script: a module, which reads the releases from a file
// of the artifact's own and draws them.
import { release } from "./render.js";

const main = document.getElementById("releases");
try {
  const data = await (await fetch("data/releases.json")).json();
  main.replaceChildren(...data.releases.map(release));
  document.getElementById("count").textContent = `${data.releases.length} releases, from data/releases.json`;
} catch (e) {
  main.textContent = "The releases couldn't be read: " + e;
}
