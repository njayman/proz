package cmd

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/njayman/proz/core"
)

// linksJSON decodes the links of the project a command printed as JSON.
func linksJSON(t *testing.T, out string) []core.Link {
	t.Helper()
	var p struct {
		Links []core.Link `json:"links"`
	}
	if err := json.Unmarshal([]byte(out), &p); err != nil || p.Links == nil {
		t.Fatalf("%v: %q", err, out)
	}
	return p.Links
}

// Agents parse the links array on every project; field names are a contract.
func TestLinkAndUnlinkAKnownTool(t *testing.T) {
	dir := configDir(t)
	web := filepath.Join(dir, "web")
	mkdirs(t, filepath.Join(web, "src"))
	run(t, web) // no terminal: registered with no program
	repo := core.Link{Kind: "tracker", Provider: "github", Ref: "https://github.com/acme/web"}
	page := core.Link{Kind: "tracker", Provider: "notion", Ref: "https://www.notion.so/acme/Tasks-89ab"}

	t.Chdir(filepath.Join(web, "src"))
	if got := linksJSON(t, run(t, "link", "HTTPS://GitHub.com/acme/web/", "--json")); !reflect.DeepEqual(got, []core.Link{repo}) {
		t.Fatalf("link here: %+v", got)
	}
	t.Chdir(dir)
	if out := run(t, "link", page.Ref, "web", "--kind", "tracker"); out != "Linked web to notion, as a tracker: "+page.Ref+"\n" {
		t.Fatalf("plain: %q", out)
	}
	want := `"links": [
    {
      "kind": "tracker",
      "provider": "github",
      "ref": "https://github.com/acme/web",
      "missing": false
    },
    {
      "kind": "tracker",
      "provider": "notion",
      "ref": "https://www.notion.so/acme/Tasks-89ab",
      "missing": false
    }
  ]`
	if out := run(t, "status", "web", "--json"); !strings.Contains(out, want) {
		t.Fatalf("status --json:\n%s", out)
	}
	var list []struct {
		Links []core.Link `json:"links"`
	}
	json.Unmarshal([]byte(run(t, "list", "--json")), &list)
	if len(list) != 1 || !reflect.DeepEqual(list[0].Links, []core.Link{repo, page}) {
		t.Fatalf("list --json: %+v", list)
	}

	if got := linksJSON(t, run(t, "unlink", "https://github.com/acme/web", "web", "--json")); !reflect.DeepEqual(got, []core.Link{page}) {
		t.Fatalf("unlink: %+v", got)
	}
	if out := run(t, "unlink", page.Ref, "web"); out != "Unlinked "+page.Ref+" from web.\n" {
		t.Fatalf("plain unlink: %q", out)
	}
	for _, args := range [][]string{
		{"unlink", page.Ref, "web"}, // no longer linked
		{"link", page.Ref},          // dir is in no project
		{"link", page.Ref, "nope"},  // no such project
		{"link", page.Ref, "web", "--kind", "tasks"},
	} {
		if execute(args...) == nil {
			t.Errorf("%v: want an error", args)
		}
	}
}
