import { scope } from "../../keymap/commands";

// verifyKeys are the Verify tab's keys (Verify.tsx binds them while it's
// open), for the bundle that's open.
export const verifyKeys = scope("mission-verify", "Verify", {
  approve: { title: "Approve check", keys: "y", row: "Approve / reject check" },
  reject: { title: "Reject check", keys: "n", row: "Approve / reject check" },
  settle: { title: "Approve bundle or send back", keys: "mod+Enter" },
});
