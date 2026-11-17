package ops

import "github.com/withworkspace/workspace/internal/github"

func LookupPR(prs map[string]*github.PR, branch, name string) *github.PR {
	if branch != "" {
		if pr := prs[branch]; pr != nil {
			return pr
		}
	}
	return prs[name]
}
