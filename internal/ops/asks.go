package ops

import (
	"os"
	"regexp"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
)

// Asking from an ask bar (web/src/ask/): a question about something as
// you read it, answered by an agent as a conversation is, and kept only
// if you keep it (conversation/ask.go). What it's about is a document
// outside any mission, as the reader has it, and a passage of it you've
// picked out; a pull request, a mission or a space can come later.

// Asked is what's asked, and what about: a document's ref (repo:path, or
// repo/capsule:path), and the passage picked out of it as it reads, with
// the heading of the section it's in.
type Asked struct {
	Doc     string
	Quote   *conversation.Quote
	Section string
	Body    string
}

// excerptMax is how many of the document's lines an ask keeps of where its
// passage is.
const excerptMax = 40

// Ask starts an ask, and has its agent answer.
func (w *Workspace) Ask(a Asked, by Actor) (*conversation.Conversation, error) {
	if strings.TrimSpace(a.Body) == "" {
		return nil, &Invalid{Reason: "say something"}
	}
	if strings.TrimSpace(a.Doc) == "" {
		return nil, &Invalid{Reason: "say what it's about: a document's ref"}
	}
	d, err := w.FindDoc(a.Doc)
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(d.Path)
	if err != nil {
		return nil, err
	}
	if _, _, err := Roles(w.Config()).ForConversation(); err != nil {
		return nil, &Invalid{Reason: "no agent can answer here: " + err.Error()}
	}
	anchor := conversation.Anchor{Doc: d.Ref, Path: d.Rel, Version: doc.Version(text), Section: strings.TrimSpace(a.Section)}
	if a.Quote != nil && strings.TrimSpace(a.Quote.Exact) != "" {
		q := *a.Quote
		anchor.Quote = &q
		if line, end, ok := QuoteLines(text, q.Exact); ok {
			lines := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
			anchor.Line, anchor.EndLine = line, end
			anchor.Excerpt = lines[line-1 : min(end, line-1+excerptMax)]
		}
	}
	c, err := conversation.ForWorkspace(w.Root()).OpenAsk(anchor, by.By(), a.Body, w.now())
	if err != nil {
		return nil, &Invalid{Reason: err.Error()}
	}
	w.answer(job.Answer, doc.Scope{}, c.ID)
	return c, nil
}

// KeepAsk keeps an ask (Keep as a conversation): it's listed with the
// rest of the conversations from then on, about its document, and isn't
// swept.
func (w *Workspace) KeepAsk(sc doc.Scope, id string, by Actor) (*conversation.Conversation, error) {
	store, err := w.Conversations(sc)
	if err != nil {
		return nil, err
	}
	return store.Keep(id, by.By(), w.now())
}

// QuoteLines is where a passage, as it reads drawn, is in a document's
// markdown: its first and last lines, numbered from 1. It's found as
// written, or else by its first and last words with only the markdown's
// marks and spacing between them (a passage reads without its `code`,
// *emphasis* or [links](…)). ok is false when it can't be found.
func QuoteLines(text []byte, exact string) (line, end int, ok bool) {
	src := strings.ReplaceAll(string(text), "\r\n", "\n")
	exact = strings.TrimSpace(exact)
	if exact == "" {
		return 0, 0, false
	}
	start, stop := -1, -1
	if i := strings.Index(src, exact); i >= 0 {
		start, stop = i, i+len(exact)
	} else {
		words := quoteWords.FindAllString(exact, -1)
		if len(words) == 0 {
			return 0, 0, false
		}
		head, tail := words[:min(len(words), 8)], words[max(0, len(words)-8):]
		loc := wordsAt(head).FindStringIndex(src)
		if loc == nil {
			return 0, 0, false
		}
		start, stop = loc[0], loc[1]
		if len(words) > len(head) {
			if t := wordsAt(tail).FindStringIndex(src[start:]); t != nil && start+t[1] > stop {
				stop = start + t[1]
			}
		}
	}
	line = strings.Count(src[:start], "\n") + 1
	end = line + strings.Count(src[start:stop], "\n")
	return line, end, true
}

var quoteWords = regexp.MustCompile(`[\p{L}\p{N}]+`)

// wordsAt matches words in order with nothing but marks and spacing
// between them (no other letters or digits).
func wordsAt(words []string) *regexp.Regexp {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = regexp.QuoteMeta(w)
	}
	return regexp.MustCompile(`(?i)` + strings.Join(quoted, `[^\p{L}\p{N}]+`))
}
