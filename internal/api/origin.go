package api

import "strings"

// Origin is what the pipeline knows about why a provide runs: where the build
// ran, the pull or merge request behind it, the tickets it closes and one line
// of intent. Sanshain stores it with the version (the latest provide that
// carries any of it wins; a byte-identical re-provide only fills an empty one).
// Embedded into the provide payloads; every field is omitted when unset, since
// the server refuses explicit nulls on a payload that denies unknown shapes.
type Origin struct {
	BuildURL  string   `json:"build_url,omitempty"`
	ChangeURL string   `json:"change_url,omitempty"`
	Tickets   []string `json:"tickets,omitempty"`
	Note      string   `json:"note,omitempty"`
}

// ResolveOrigin resolves the origin of this build: an explicit flag first, then
// SANSHAIN_BUILD_URL / SANSHAIN_CHANGE_URL / SANSHAIN_TICKETS / SANSHAIN_NOTE,
// and for the build and change URL the runner's own variables on GitHub
// Actions, GitLab CI and Jenkins. Tickets and the note are human intent and
// never come from the runner. Nothing is read from git.
func ResolveOrigin(buildURL, changeURL, tickets, note string, getenv func(string) string) Origin {
	o := Origin{
		BuildURL:  first(buildURL, getenv("SANSHAIN_BUILD_URL"), runnerBuildURL(getenv)),
		ChangeURL: first(changeURL, getenv("SANSHAIN_CHANGE_URL"), runnerChangeURL(getenv)),
		Note:      first(note, getenv("SANSHAIN_NOTE")),
	}
	for _, raw := range strings.Split(first(tickets, getenv("SANSHAIN_TICKETS")), ",") {
		id := strings.TrimSpace(raw)
		if id == "" || contains(o.Tickets, id) {
			continue
		}
		o.Tickets = append(o.Tickets, id)
	}
	return o
}

// IsZero reports whether the origin says nothing.
func (o Origin) IsZero() bool {
	return o.BuildURL == "" && o.ChangeURL == "" && len(o.Tickets) == 0 && o.Note == ""
}

func runnerBuildURL(getenv func(string) string) string {
	server, repo, run := getenv("GITHUB_SERVER_URL"), getenv("GITHUB_REPOSITORY"), getenv("GITHUB_RUN_ID")
	if present(server) && present(repo) && present(run) {
		return strings.TrimSpace(server) + "/" + strings.TrimSpace(repo) + "/actions/runs/" + strings.TrimSpace(run)
	}
	return first(getenv("CI_JOB_URL"), getenv("BUILD_URL"))
}

func runnerChangeURL(getenv func(string) string) string {
	server, repo, ref := getenv("GITHUB_SERVER_URL"), getenv("GITHUB_REPOSITORY"), strings.TrimSpace(getenv("GITHUB_REF"))
	if present(server) && present(repo) && strings.HasPrefix(ref, "refs/pull/") {
		parts := strings.Split(ref, "/")
		if len(parts) >= 3 && parts[2] != "" {
			return strings.TrimSpace(server) + "/" + strings.TrimSpace(repo) + "/pull/" + parts[2]
		}
	}
	project, iid := getenv("CI_MERGE_REQUEST_PROJECT_URL"), getenv("CI_MERGE_REQUEST_IID")
	if present(project) && present(iid) {
		return strings.TrimSpace(project) + "/-/merge_requests/" + strings.TrimSpace(iid)
	}
	return first(getenv("CHANGE_URL"))
}

func first(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

func present(v string) bool { return strings.TrimSpace(v) != "" }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
