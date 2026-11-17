// Package acp is a client for the Agent Client Protocol: JSON-RPC 2.0 over
// an agent's stdin and stdout, one message per line. ws is the client (the
// editor, in ACP's terms): it starts a session, sends prompts, and hears
// back what the agent says and does as session updates, answering what the
// agent asks of it (leave to make a tool call, a form to fill in).
//
// It knows ACP, not any one agent. What an agent adds of its own (Cursor's
// cursor/ask_question, say) comes to the caller as an extension to answer,
// and a harness builds on these pieces for its agent.
//
// The pieces, lowest first:
//
//   - Conn: JSON-RPC over a pair of streams.
//   - Client: one agent process, spoken to over a Conn.
//   - Transcript: a session's updates folded into what happened, in order.
//   - Record: a session's events as JSON lines, written as they happen and
//     read back into a transcript.
//   - Asks: what the agent is waiting on the client for, held until someone
//     (a person, or a policy) answers.
//   - Agent: one session run: prompts sent a turn at a time, with its asks,
//     transcript and record kept as it goes.
package acp

// ProtocolVersion is the version of ACP this client speaks.
const ProtocolVersion = 1
