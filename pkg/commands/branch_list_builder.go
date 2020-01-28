package commands

import (
	"regexp"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/utils"

	"github.com/sirupsen/logrus"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

// context:
// we want to only show 'safe' branches (ones that haven't e.g. been deleted)
// which `git branch -a` gives us, but we also want the recency data that
// git reflog gives us.
// So we get the HEAD, then append get the reflog branches that intersect with
// our safe branches, then add the remaining safe branches, ensuring uniqueness
// along the way

// if we find out we need to use one of these functions in the git.go file, we
// can just pull them out of here and put them there and then call them from in here

// BranchListBuilder returns a list of Branch objects for the current repo
type BranchListBuilder struct {
	Log        *logrus.Entry
	GitCommand *GitCommand
}

// NewBranchListBuilder builds a new branch list builder
func NewBranchListBuilder(log *logrus.Entry, gitCommand *GitCommand) (*BranchListBuilder, error) {
	return &BranchListBuilder{
		Log:        log,
		GitCommand: gitCommand,
	}, nil
}

func (b *BranchListBuilder) obtainCurrentBranch() *Branch {
	branchName, err := b.GitCommand.CurrentBranchName()
	if err != nil {
		panic(err.Error())
	}

	return &Branch{Name: strings.TrimSpace(branchName)}
}

func (b *BranchListBuilder) obtainReflogBranches() []*Branch {
	branches := make([]*Branch, 0)
	// if we directly put this string in RunCommandWithOutput the compiler complains because it thinks it's a format string
	unescaped := "git reflog --date=relative --pretty='%gd|%gs' --grep-reflog='checkout: moving' HEAD"
	rawString, err := b.GitCommand.OSCommand.RunCommandWithOutput(unescaped)
	if err != nil {
		return branches
	}

	branchLines := utils.SplitLines(rawString)
	for _, line := range branchLines {
		recency, branchName := branchInfoFromLine(line)
		if branchName == "" {
			continue
		}
		branch := &Branch{Name: branchName, Recency: recency}
		branches = append(branches, branch)
	}
	return uniqueByName(branches)
}

func (b *BranchListBuilder) obtainSafeBranches() []*Branch {
	branches := make([]*Branch, 0)

	bIter, err := b.GitCommand.Repo.Branches()
	if err != nil {
		panic(err)
	}
	bIter.ForEach(func(b *plumbing.Reference) error {
		name := b.Name().Short()
		branches = append(branches, &Branch{Name: name})
		return nil
	})

	return branches
}

func (b *BranchListBuilder) appendNewBranches(finalBranches, newBranches, existingBranches []*Branch, included bool) []*Branch {
	for _, newBranch := range newBranches {
		if included == branchIncluded(newBranch.Name, existingBranches) {
			finalBranches = append(finalBranches, newBranch)
		}
	}
	return finalBranches
}

func sanitisedReflogName(reflogBranch *Branch, safeBranches []*Branch) string {
	for _, safeBranch := range safeBranches {
		if strings.ToLower(safeBranch.Name) == strings.ToLower(reflogBranch.Name) {
			return safeBranch.Name
		}
	}
	return reflogBranch.Name
}

// Build the list of branches for the current repo
func (b *BranchListBuilder) Build() []*Branch {
	branches := make([]*Branch, 0)
	head := b.obtainCurrentBranch()
	safeBranches := b.obtainSafeBranches()

	reflogBranches := b.obtainReflogBranches()
	for i, reflogBranch := range reflogBranches {
		reflogBranches[i].Name = sanitisedReflogName(reflogBranch, safeBranches)
	}

	branches = b.appendNewBranches(branches, reflogBranches, safeBranches, true)
	branches = b.appendNewBranches(branches, safeBranches, branches, false)

	if len(branches) == 0 || branches[0].Name != head.Name {
		branches = append([]*Branch{head}, branches...)
	}

	branches[0].Recency = "  *"

	return branches
}

func branchIncluded(branchName string, branches []*Branch) bool {
	for _, existingBranch := range branches {
		if strings.ToLower(existingBranch.Name) == strings.ToLower(branchName) {
			return true
		}
	}
	return false
}

func uniqueByName(branches []*Branch) []*Branch {
	finalBranches := make([]*Branch, 0)
	for _, branch := range branches {
		if branchIncluded(branch.Name, finalBranches) {
			continue
		}
		finalBranches = append(finalBranches, branch)
	}
	return finalBranches
}

// A line will have the form '10 days ago master' so we need to strip out the
// useful information from that into timeNumber, timeUnit, and branchName
func branchInfoFromLine(line string) (string, string) {
	// example line: HEAD@{2020-01-28 20:55:06 +1100}|checkout: moving from pulling-from-forks to tim77-patch-1
	r := regexp.MustCompile(`HEAD\@\{([^\s]+) ([^\s]+) ago\}\|.*?([^\s]*)$`)
	matches := r.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) == 0 {
		return "", ""
	}
	since := matches[1]
	unit := matches[2]
	branchName := matches[3]
	return since + abbreviatedTimeUnit(unit), branchName
}

func abbreviatedTimeUnit(timeUnit string) string {
	r := regexp.MustCompile("s$")
	timeUnit = r.ReplaceAllString(timeUnit, "")
	timeUnitMap := map[string]string{
		"hour":   "h",
		"minute": "m",
		"second": "s",
		"week":   "w",
		"year":   "y",
		"day":    "d",
		"month":  "m",
	}
	return timeUnitMap[timeUnit]
}
