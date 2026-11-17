package team

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/slug"
)

// One person has two names: a login on GitHub, and a name and email in
// git. They're paired through what one event has both of (a squash
// merge's commit), through GitHub's no-reply addresses, which carry the
// login, and by what [team.people] says outright.

// Person is someone who's done something in the workspace's repos.
type Person struct {
	// Key names them: [team.people]'s, or their login, or their name.
	Key  string `json:"key"`
	Name string `json:"name"`
	// Short is what they're called in a sentence: their first name, or
	// their whole name where two people share it.
	Short string `json:"short"`
	Login string `json:"login,omitempty"`
	Bot   bool   `json:"bot,omitempty"`
}

// PersonConfig is one of [team.people]: who someone is, said outright.
type PersonConfig struct {
	Name   string
	Logins []string
	Emails []string
}

// Roster is the people some events are by.
type Roster struct {
	people []Person
	of     map[string]int // an identity → its person
}

// noReply is a GitHub no-reply address: 12345+login@… or login@….
var noReply = regexp.MustCompile(`^(?:\d+\+)?([^@+]+)@users\.noreply\.github\.com$`)

func loginID(s string) string { return "l:" + strings.ToLower(s) }
func emailID(s string) string { return "e:" + strings.ToLower(s) }
func nameID(s string) string  { return "n:" + strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// identities are the names an event knows its person by.
func identities(e Event) []string {
	var ids []string
	if e.Login != "" {
		// Someone whose name in git is their login is one person.
		ids = append(ids, loginID(e.Login), nameID(e.Login))
	}
	if e.Email != "" {
		ids = append(ids, emailID(e.Email))
		if m := noReply.FindStringSubmatch(strings.ToLower(e.Email)); m != nil {
			ids = append(ids, loginID(m[1]))
		}
	}
	if strings.TrimSpace(e.Name) != "" {
		ids = append(ids, nameID(e.Name))
	}
	return ids
}

// Resolve works out who the events are by.
func Resolve(events []Event, cfg map[string]PersonConfig) *Roster {
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		p, ok := parent[x]
		if !ok {
			parent[x] = x
			return x
		}
		if p == x {
			return x
		}
		root := find(p)
		parent[x] = root
		return root
	}
	join := func(ids []string) {
		for i := range ids {
			find(ids[i])
			if i > 0 {
				parent[find(ids[i])] = find(ids[0])
			}
		}
	}
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := cfg[k]
		ids := []string{"k:" + k}
		for _, l := range c.Logins {
			ids = append(ids, loginID(l))
		}
		for _, m := range c.Emails {
			ids = append(ids, emailID(m))
		}
		if c.Name != "" {
			ids = append(ids, nameID(c.Name))
		}
		join(ids)
	}
	for _, e := range events {
		join(identities(e))
	}

	// What's known of each set: its key and name from the config, the
	// names and logins its events use (the most used wins), a bot.
	type facts struct {
		key, name string
		names     map[string]int
		logins    map[string]int
		emails    map[string]int
		bot       bool
		first     int
	}
	sets := map[string]*facts{}
	set := func(root string, i int) *facts {
		f := sets[root]
		if f == nil {
			f = &facts{names: map[string]int{}, logins: map[string]int{}, emails: map[string]int{}, first: i}
			sets[root] = f
		}
		return f
	}
	for _, k := range keys {
		f := set(find("k:"+k), -1)
		if f.key == "" {
			f.key, f.name = k, cfg[k].Name
		}
		for _, l := range cfg[k].Logins {
			f.logins[l] += 0
		}
	}
	for i, e := range events {
		ids := identities(e)
		if len(ids) == 0 {
			continue
		}
		f := set(find(ids[0]), i)
		if n := strings.Join(strings.Fields(e.Name), " "); n != "" {
			f.names[n]++
		}
		if e.Login != "" {
			f.logins[e.Login]++
		}
		if e.Email != "" {
			f.emails[strings.ToLower(e.Email)]++
		}
		f.bot = f.bot || e.Bot
	}

	r := &Roster{of: map[string]int{}}
	roots := make([]string, 0, len(sets))
	for root := range sets {
		roots = append(roots, root)
	}
	sort.Slice(roots, func(i, j int) bool {
		if a, b := sets[roots[i]].first, sets[roots[j]].first; a != b {
			return a < b
		}
		return roots[i] < roots[j]
	})
	index := map[string]int{}
	taken := map[string]bool{}
	for _, root := range roots {
		f := sets[root]
		p := Person{Key: f.key, Name: f.name, Login: most(f.logins), Bot: f.bot}
		if p.Name == "" {
			// A whole name before a first name alone, where there's one.
			whole := map[string]int{}
			for name, n := range f.names {
				if strings.Contains(name, " ") {
					whole[name] = n
				}
			}
			if p.Name = most(whole); p.Name == "" {
				p.Name = most(f.names)
			}
		}
		if p.Name == "" {
			p.Name = p.Login
		}
		if p.Name == "" {
			p.Name = most(f.emails)
		}
		if p.Key == "" {
			p.Key = strings.ToLower(p.Login)
		}
		if p.Key == "" {
			p.Key = personSlug(p.Name)
		}
		for base, n := p.Key, 2; taken[p.Key]; n++ {
			p.Key = base + "-" + strconv.Itoa(n)
		}
		taken[p.Key] = true
		index[root] = len(r.people)
		r.people = append(r.people, p)
	}
	for id := range parent {
		if i, ok := index[find(id)]; ok {
			r.of[id] = i
		}
	}
	// First names, where they tell people apart.
	firsts := map[string]int{}
	for _, p := range r.people {
		firsts[firstName(p.Name)]++
	}
	for i := range r.people {
		p := &r.people[i]
		if first := firstName(p.Name); firsts[first] == 1 && !p.Bot {
			p.Short = first
		} else {
			p.Short = p.Name
		}
	}
	return r
}

// Of is who an event is by: nil if it says nobody.
func (r *Roster) Of(e Event) *Person {
	for _, id := range identities(e) {
		if i, ok := r.of[id]; ok {
			return &r.people[i]
		}
	}
	return nil
}

// People is everyone, in the order they first did something.
func (r *Roster) People() []Person { return r.people }

func most(counts map[string]int) string {
	best := ""
	for s, n := range counts {
		if best == "" || n > counts[best] || (n == counts[best] && s < best) {
			best = s
		}
	}
	return best
}

func firstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}
	return name
}

func personSlug(s string) string { return slug.Make(s, 0, "someone") }
