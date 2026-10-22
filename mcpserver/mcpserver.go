// Package mcpserver serves proz to agents over MCP. It runs as the agent's
// child process over stdio: no network listener, no auth. Tools call core
// and return the same JSON shapes as the CLI's --json (package contract).
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/njayman/proz/contract"
	"github.com/njayman/proz/core"
)

// Target names a project: by name, or by any directory inside its folder.
type Target struct {
	Project string `json:"project,omitempty" jsonschema:"the project's name"`
	Dir     string `json:"dir,omitempty" jsonschema:"an absolute path inside the project's folder, such as your working directory; used when project is empty"`
}

type (
	noInput   struct{}
	ideaInput struct {
		Text string `json:"text" jsonschema:"the idea in one line"`
	}
	milestoneInput struct {
		Target
		Text string `json:"text" jsonschema:"what is done and what comes next, in one sentence, e.g. 'shipped login; next: signup'"`
	}
	linkInput struct { // provider's choices are core.Providers; a tag must be a constant
		Target
		Ref      string `json:"ref" jsonschema:"the web address of the project's tracker or notes (such as its GitHub repository, Linear project or Notion page), owner/repo for GitHub, or an absolute path to a local notes file or folder"`
		Provider string `json:"provider,omitempty" jsonschema:"only when proz does not recognize the address: github, linear, jira, trello, asana, todoist, notion, brainmd or other"`
		Kind     string `json:"kind,omitempty" jsonschema:"tracker (where its tasks live) or brain (where its notes live); defaults to the tool's usual kind"`
	}
	named struct {
		Name string `json:"name"`
	}
	projectsOut struct {
		Projects []contract.Status `json:"projects"`
	}
	nudgesOut struct {
		Nudges []contract.Nudge `json:"nudges"`
	}
)

// New returns the proz MCP server. open opens the store for one call, so the
// server holds no connection between calls, like the CLI.
func New(open func() (*core.Store, error), version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "proz", Version: version}, nil)
	with := func(fn func(*core.Store) error) error {
		st, err := open()
		if err != nil {
			return err
		}
		defer st.Close()
		return fn(st)
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_projects",
		Description: "List every project proz tracks, each with its state (active, stale, snoozed, archived), last activity, left-off note and milestones.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, projectsOut, error) {
		out := projectsOut{Projects: []contract.Status{}}
		err := with(func(st *core.Store) error {
			all, err := st.Statuses()
			if err != nil {
				return err
			}
			for _, p := range all {
				ms, err := st.Milestones(p.ID)
				if err != nil {
					return err
				}
				out.Projects = append(out.Projects, contract.NewStatus(p, ms))
			}
			return nil
		})
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_project",
		Description: "Get one project: where it left off (the note), its state, last activity and milestones. Use it before resuming work on a project.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in Target) (*mcp.CallToolResult, contract.Status, error) {
		var out contract.Status
		err := with(func(st *core.Store) error {
			p, err := resolve(st, in)
			if err != nil {
				return err
			}
			out, err = status(st, p.ID)
			return err
		})
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "record_milestone",
		Description: "Record a milestone on a project: what is done and what comes next. It leads the project's left-off note when the user returns. " +
			"Record one at the end of a work session, when the user pauses or switches away, or after finishing a meaningful piece of work; not per message.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in milestoneInput) (*mcp.CallToolResult, named, error) {
		var out named
		err := with(func(st *core.Store) error {
			p, err := resolve(st, in.Target)
			if err != nil {
				return err
			}
			out.Name = p.Name
			return st.AddMilestone(p.ID, in.Text)
		})
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "capture_idea",
		Description: "Capture an idea the user is not acting on now as a project with no folder, so it resurfaces when it goes quiet.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in ideaInput) (*mcp.CallToolResult, named, error) {
		var out named
		err := with(func(st *core.Store) (err error) { out.Name, err = st.AddIdea(in.Text); return err })
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_nudges",
		Description: "List the projects the user has let go quiet (stale, not snoozed or dropped), each with its left-off note. Use it when the user asks what they forgot or what to pick up next.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, nudgesOut, error) {
		var out nudgesOut
		err := with(func(st *core.Store) error {
			ns, err := st.Nudges()
			out.Nudges = contract.Nudges(ns)
			return err
		})
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "add_link",
		Description: "Link a project to the tracker or brain where its tasks or notes live, like 'proz link', when you see the user working in a tool the project has no link for. " +
			"proz never reads through a link; you read the tool with your own MCP server for it. An address proz does not recognize fails naming the providers and kinds: retry with provider and kind. Returns the project as get_project does.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in linkInput) (*mcp.CallToolResult, contract.Status, error) {
		var out contract.Status
		err := with(func(st *core.Store) error {
			p, err := resolve(st, in.Target)
			if err != nil {
				return err
			}
			if _, err := st.AddLink(p.ID, in.Ref, in.Provider, in.Kind); err != nil {
				return err
			}
			out, err = status(st, p.ID)
			return err
		})
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "read_brain_note",
		Description: "Read a project's note from the user's brain (a local markdown folder): proz's section with the left-off note and milestones, " +
			"plus anything the user wrote around it. Local files only, no network; it first refreshes proz's section in the note. Fails if the user has not set a brain folder.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in Target) (*mcp.CallToolResult, contract.BrainNote, error) {
		var out contract.BrainNote
		err := with(func(st *core.Store) error {
			p, err := resolve(st, in)
			if err != nil {
				return err
			}
			out.Path, out.Content, err = st.BrainNote(p.ID)
			return err
		})
		return nil, out, err
	})
	return s
}

// status is project id in full, as get_project returns it.
func status(st *core.Store, id int64) (contract.Status, error) {
	s, err := st.Status(id)
	if err != nil {
		return contract.Status{}, err
	}
	ms, err := st.Milestones(id)
	return contract.NewStatus(s, ms), err
}

func resolve(st *core.Store, t Target) (core.Project, error) {
	var p core.Project
	var err error
	switch {
	case t.Project != "" && t.Dir != "":
		return p, errors.New("pass project or dir, not both")
	case t.Project != "":
		p, err = st.ProjectNamed(t.Project)
	case t.Dir != "":
		if !filepath.IsAbs(t.Dir) {
			return p, fmt.Errorf("dir must be an absolute path, got %q", t.Dir)
		}
		p, err = st.ProjectAt(t.Dir)
	default:
		return p, errors.New("pass project (a name) or dir (a path inside the project)")
	}
	if errors.Is(err, core.ErrNotFound) {
		return p, fmt.Errorf("no proz project matches %q", t.Project+t.Dir)
	}
	return p, err
}
