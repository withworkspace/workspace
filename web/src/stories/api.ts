// The API as the stories have it: what was taken with their missions
// answers a GET, and nothing can be written.
//
// A story draws a part of the web UI alone, with a mission it's given. The
// part may still ask the API (the repos, for what a name links to; crew's
// transcript, for what it's stopped on). serve says what a path answers;
// anything else isn't there, and a write is refused in words the page
// shows, so a button pressed in a story says why nothing happened.
const answers = new Map<string, unknown>();

export function serve(path: string, body: unknown) {
  answers.set(path, body);
}

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

export function stubApi() {
  const real = window.fetch.bind(window);
  window.fetch = async (input, init) => {
    const url = new URL(typeof input === "string" ? input : input instanceof URL ? input.href : input.url, window.location.origin);
    if (!url.pathname.startsWith("/api/")) return real(input, init);
    if ((init?.method ?? "GET").toUpperCase() !== "GET") return json(409, { error: "This is a story: nothing's written from here." });
    const hit = answers.get(url.pathname + url.search) ?? answers.get(url.pathname);
    return hit === undefined ? json(404, { error: "Not among what the stories took." }) : json(200, hit);
  };
}
