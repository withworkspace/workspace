package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// ws silo watch shows what the daemon's doing with the silos: it asks how
// they stand every siloWatchEvery.
const siloWatchEvery = 500 * time.Millisecond

// siloSyncsShown is how many of each silo's last syncs the screen shows.
const siloSyncsShown = 3

type siloWatchModel struct {
	formatRepoName func(string) string
	ask            func() ([]workspace.SiloState, error)
	resync         func() error
	silos          []workspace.SiloState
	err            error
	spinner        spinner.Model
	resyncing      bool
}

type silosMsg struct {
	silos []workspace.SiloState
	err   error
}
type resyncDoneMsg struct{ err error }

func newSiloWatchModel(formatRepoName func(string) string, ask func() ([]workspace.SiloState, error), resync func() error) siloWatchModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return siloWatchModel{formatRepoName: formatRepoName, ask: ask, resync: resync, spinner: s}
}

func (m siloWatchModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.poll(0))
}

func (m siloWatchModel) poll(after time.Duration) tea.Cmd {
	ask := m.ask
	return func() tea.Msg {
		time.Sleep(after)
		silos, err := ask()
		return silosMsg{silos, err}
	}
}

func (m siloWatchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case silosMsg:
		m.err = msg.err
		if msg.err == nil {
			m.silos = msg.silos
		}
		return m, m.poll(siloWatchEvery)

	case resyncDoneMsg:
		m.resyncing = false
		m.err = msg.err
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			if !m.resyncing {
				m.resyncing = true
				resync := m.resync
				return m, func() tea.Msg { return resyncDoneMsg{resync()} }
			}
		}
	}
	return m, nil
}

func (m siloWatchModel) View() tea.View { return tea.NewView(m.render(time.Now())) }

func (m siloWatchModel) render(now time.Time) string {
	var b strings.Builder

	header := fmt.Sprintf("  %s The daemon's keeping your silos", m.spinner.View())
	if m.resyncing {
		header += fmt.Sprintf("  %s", ui.Orange.Render("resyncing..."))
	}
	b.WriteString(header + "\n")
	b.WriteString(fmt.Sprintf("  %s\n\n", ui.Dim.Render("r to resync, q to quit (they're kept either way)")))
	if m.err != nil {
		b.WriteString(fmt.Sprintf("  %s %v\n\n", ui.Orange.Render("⚠"), m.err))
	}

	for _, st := range m.silos {
		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			ui.Bold.Render(m.formatRepoName(st.Repo)),
			ui.Dim.Render("→"),
			ui.TagDim.Render(st.Target)))
		if st.Error != "" {
			b.WriteString(fmt.Sprintf("    %s\n", ui.Orange.Render(st.Error)))
		}
		syncs := st.Syncs
		if len(syncs) > siloSyncsShown {
			syncs = syncs[len(syncs)-siloSyncsShown:]
		}
		if len(syncs) == 0 && st.Error == "" {
			b.WriteString(fmt.Sprintf("    %s\n", ui.Dim.Render("waiting for changes...")))
		}
		for _, s := range syncs {
			b.WriteString("    " + siloSyncLine(s, true) + "\n")
		}
		if h := st.Hook; h != nil {
			b.WriteString("    " + siloHookLine(h, now, true) + "\n")
		}
		b.WriteByte('\n')
	}

	return b.String()
}

// siloSyncLine is a sync in a line.
func siloSyncLine(s workspace.SiloSync, styled bool) string {
	style := func(r func(...string) string, t string) string {
		if styled {
			return r(t)
		}
		return t
	}
	what := fmt.Sprintf("%d file(s)", s.Files)
	if s.Full {
		what += " (" + s.Why + ")"
	}
	if s.Error != "" {
		return fmt.Sprintf("%s %s %s: %s", style(ui.Dim.Render, s.At.Format("15:04:05")), style(ui.Orange.Render, "⚠"), what, s.Error)
	}
	return fmt.Sprintf("%s %s %s", style(ui.Dim.Render, s.At.Format("15:04:05")), style(ui.Green.Render, "✓"), what)
}

// siloHookLine is the last hook a silo ran, in a line.
func siloHookLine(h *workspace.SiloHook, now time.Time, styled bool) string {
	if h.Error != "" {
		said := fmt.Sprintf("%s hook failed %s ago: %s", h.Name, ago(now, h.At), h.Error)
		if styled {
			return ui.Orange.Render(said)
		}
		return said
	}
	said := fmt.Sprintf("%s hook ran %s ago", h.Name, ago(now, h.At))
	if styled {
		return ui.Dim.Render(said)
	}
	return said
}

// printSiloSyncs prints a line for each sync and hook the daemon makes
// from now, until stop.
func printSiloSyncs(w io.Writer, ask func() ([]workspace.SiloState, error), stop <-chan os.Signal) error {
	seen := map[string]time.Time{}
	hooks := map[string]time.Time{}
	first := true
	t := time.NewTicker(siloWatchEvery)
	defer t.Stop()
	for {
		silos, err := ask()
		if err != nil {
			return err
		}
		for _, st := range silos {
			for _, s := range st.Syncs {
				if s.At.After(seen[st.Repo]) {
					seen[st.Repo] = s.At
					if !first {
						fmt.Fprintf(w, "%s: %s\n", st.Repo, siloSyncLine(s, false))
					}
				}
			}
			if h := st.Hook; h != nil && h.At.After(hooks[st.Repo]) {
				hooks[st.Repo] = h.At
				if !first {
					fmt.Fprintf(w, "%s: %s\n", st.Repo, siloHookLine(h, time.Now(), false))
				}
			}
		}
		if first {
			fmt.Fprintf(w, "The daemon's keeping %d silo(s); a line for each sync from now. Ctrl+C to stop.\n", len(silos))
			first = false
		}
		select {
		case <-stop:
			return nil
		case <-t.C:
		}
	}
}
