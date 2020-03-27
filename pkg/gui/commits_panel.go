package gui

import (
	"strconv"
	"sync"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/commands"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// list panel functions

func (gui *Gui) getSelectedCommit(g *gocui.Gui) *commands.Commit {
	selectedLine := gui.State.Panels.Commits.SelectedLine
	if selectedLine == -1 {
		return nil
	}

	return gui.State.Commits[selectedLine]
}

func (gui *Gui) handleCommitSelect(g *gocui.Gui, v *gocui.View) error {
	if gui.popupPanelFocused() {
		return nil
	}

	// this probably belongs in an 'onFocus' function than a 'commit selected' function
	if err := gui.refreshSecondaryPatchPanel(); err != nil {
		return err
	}

	if _, err := gui.g.SetCurrentView(v.Name()); err != nil {
		return err
	}

	state := gui.State.Panels.Commits
	if state.SelectedLine > 290 && state.LimitCommits {
		state.LimitCommits = false
		go func() {
			if err := gui.refreshCommitsWithLimit(); err != nil {
				_ = gui.createErrorPanel(gui.g, err.Error())
			}
		}()
	}

	gui.getMainView().Title = "Patch"
	gui.getSecondaryView().Title = "Custom Patch"
	gui.handleEscapeLineByLinePanel()

	commit := gui.getSelectedCommit(g)
	if commit == nil {
		return gui.newStringTask("main", gui.Tr.SLocalize("NoCommitsThisBranch"))
	}

	v.FocusPoint(0, gui.State.Panels.Commits.SelectedLine)

	// if specific diff mode is on, don't show diff
	if gui.State.Panels.Commits.SpecificDiffMode {
		return nil
	}

	cmd := gui.OSCommand.ExecutableFromString(
		gui.GitCommand.ShowCmdStr(commit.Sha),
	)
	if err := gui.newPtyTask("main", cmd); err != nil {
		gui.Log.Error(err)
	}

	return nil
}

// whenever we change commits, we should update branches because the upstream/downstream
// counts can change. Whenever we change branches we should probably also change commits
// e.g. in the case of switching branches. We also need the status to be refreshed whenever
// the working tree status changes or the branch upstream/downstream value changes.
// Given how fast the refreshStatus method is, we should really just call it every time
// we refresh, but I'm not sure how to do that asynchronously that prevents a race condition
// other than a mutex.
func (gui *Gui) refreshCommits() error {
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		gui.refreshReflogCommits()
		gui.refreshBranches()
		gui.refreshStatus()
		wg.Done()
	}()

	go func() {
		gui.refreshCommitsWithLimit()
		if gui.g.CurrentView() == gui.getCommitFilesView() || (gui.g.CurrentView() == gui.getMainView() && gui.State.MainContext == "patch-building") {
			gui.refreshCommitFilesView()
		}
		wg.Done()
	}()

	wg.Wait()

	return nil
}

func (gui *Gui) refreshCommitsWithLimit() error {
	builder, err := commands.NewCommitListBuilder(gui.Log, gui.GitCommand, gui.OSCommand, gui.Tr, gui.State.CherryPickedCommits, gui.State.DiffEntries)
	if err != nil {
		return err
	}

	commits, err := builder.GetCommits(gui.State.Panels.Commits.LimitCommits)
	if err != nil {
		return err
	}
	gui.State.Commits = commits

	if gui.getCommitsView().Context == "branch-commits" {
		if err := gui.renderBranchCommitsWithSelection(); err != nil {
			return err
		}
	}

	return nil
}

// specific functions

func (gui *Gui) handleCommitSquashDown(g *gocui.Gui, v *gocui.View) error {
	if len(gui.State.Commits) <= 1 {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("YouNoCommitsToSquash"))
	}

	applied, err := gui.handleMidRebaseCommand("squash")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	return gui.createConfirmationPanel(g, v, true, gui.Tr.SLocalize("Squash"), gui.Tr.SLocalize("SureSquashThisCommit"), func(g *gocui.Gui, v *gocui.View) error {
		return gui.WithWaitingStatus(gui.Tr.SLocalize("SquashingStatus"), func() error {
			err := gui.GitCommand.InteractiveRebase(gui.State.Commits, gui.State.Panels.Commits.SelectedLine, "squash")
			return gui.handleGenericMergeCommandResult(err)
		})
	}, nil)
}

func (gui *Gui) handleCommitFixup(g *gocui.Gui, v *gocui.View) error {
	if len(gui.State.Commits) <= 1 {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("YouNoCommitsToSquash"))
	}

	applied, err := gui.handleMidRebaseCommand("fixup")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	return gui.createConfirmationPanel(g, v, true, gui.Tr.SLocalize("Fixup"), gui.Tr.SLocalize("SureFixupThisCommit"), func(g *gocui.Gui, v *gocui.View) error {
		return gui.WithWaitingStatus(gui.Tr.SLocalize("FixingStatus"), func() error {
			err := gui.GitCommand.InteractiveRebase(gui.State.Commits, gui.State.Panels.Commits.SelectedLine, "fixup")
			return gui.handleGenericMergeCommandResult(err)
		})
	}, nil)
}

func (gui *Gui) handleRenameCommit(g *gocui.Gui, v *gocui.View) error {
	applied, err := gui.handleMidRebaseCommand("reword")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if gui.State.Panels.Commits.SelectedLine != 0 {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("OnlyRenameTopCommit"))
	}
	return gui.createPromptPanel(g, v, gui.Tr.SLocalize("renameCommit"), "", func(g *gocui.Gui, v *gocui.View) error {
		if err := gui.GitCommand.RenameCommit(v.Buffer()); err != nil {
			return gui.createErrorPanel(g, err.Error())
		}

		return gui.refreshSidePanels(refreshOptions{mode: ASYNC})
	})
}

func (gui *Gui) handleRenameCommitEditor(g *gocui.Gui, v *gocui.View) error {
	applied, err := gui.handleMidRebaseCommand("reword")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	subProcess, err := gui.GitCommand.RewordCommit(gui.State.Commits, gui.State.Panels.Commits.SelectedLine)
	if err != nil {
		return gui.createErrorPanel(gui.g, err.Error())
	}
	if subProcess != nil {
		gui.SubProcess = subProcess
		return gui.Errors.ErrSubProcess
	}

	return nil
}

// handleMidRebaseCommand sees if the selected commit is in fact a rebasing
// commit meaning you are trying to edit the todo file rather than actually
// begin a rebase. It then updates the todo file with that action
func (gui *Gui) handleMidRebaseCommand(action string) (bool, error) {
	selectedCommit := gui.State.Commits[gui.State.Panels.Commits.SelectedLine]
	if selectedCommit.Status != "rebasing" {
		return false, nil
	}

	// for now we do not support setting 'reword' because it requires an editor
	// and that means we either unconditionally wait around for the subprocess to ask for
	// our input or we set a lazygit client as the EDITOR env variable and have it
	// request us to edit the commit message when prompted.
	if action == "reword" {
		return true, gui.createErrorPanel(gui.g, gui.Tr.SLocalize("rewordNotSupported"))
	}

	if err := gui.GitCommand.EditRebaseTodo(gui.State.Panels.Commits.SelectedLine, action); err != nil {
		return false, gui.createErrorPanel(gui.g, err.Error())
	}
	// TODO: consider doing this in a way that is less expensive. We don't actually
	// need to reload all the commits, just the TODO commits.
	return true, gui.refreshSidePanels(refreshOptions{scope: []int{COMMITS}})
}

func (gui *Gui) handleCommitDelete(g *gocui.Gui, v *gocui.View) error {
	applied, err := gui.handleMidRebaseCommand("drop")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	return gui.createConfirmationPanel(gui.g, v, true, gui.Tr.SLocalize("DeleteCommitTitle"), gui.Tr.SLocalize("DeleteCommitPrompt"), func(*gocui.Gui, *gocui.View) error {
		return gui.WithWaitingStatus(gui.Tr.SLocalize("DeletingStatus"), func() error {
			err := gui.GitCommand.InteractiveRebase(gui.State.Commits, gui.State.Panels.Commits.SelectedLine, "drop")
			return gui.handleGenericMergeCommandResult(err)
		})
	}, nil)
}

func (gui *Gui) handleCommitMoveDown(g *gocui.Gui, v *gocui.View) error {
	index := gui.State.Panels.Commits.SelectedLine
	selectedCommit := gui.State.Commits[index]
	if selectedCommit.Status == "rebasing" {
		if gui.State.Commits[index+1].Status != "rebasing" {
			return nil
		}
		if err := gui.GitCommand.MoveTodoDown(index); err != nil {
			return gui.createErrorPanel(gui.g, err.Error())
		}
		gui.State.Panels.Commits.SelectedLine++
		return gui.refreshSidePanels(refreshOptions{mode: BLOCK_UI, scope: []int{COMMITS, BRANCHES}})
	}

	return gui.WithWaitingStatus(gui.Tr.SLocalize("MovingStatus"), func() error {
		err := gui.GitCommand.MoveCommitDown(gui.State.Commits, index)
		if err == nil {
			gui.State.Panels.Commits.SelectedLine++
		}
		return gui.handleGenericMergeCommandResult(err)
	})
}

func (gui *Gui) handleCommitMoveUp(g *gocui.Gui, v *gocui.View) error {
	index := gui.State.Panels.Commits.SelectedLine
	if index == 0 {
		return nil
	}
	selectedCommit := gui.State.Commits[index]
	if selectedCommit.Status == "rebasing" {
		if err := gui.GitCommand.MoveTodoDown(index - 1); err != nil {
			return gui.createErrorPanel(gui.g, err.Error())
		}
		gui.State.Panels.Commits.SelectedLine--
		return gui.refreshSidePanels(refreshOptions{mode: BLOCK_UI, scope: []int{COMMITS, BRANCHES}})
	}

	return gui.WithWaitingStatus(gui.Tr.SLocalize("MovingStatus"), func() error {
		err := gui.GitCommand.MoveCommitDown(gui.State.Commits, index-1)
		if err == nil {
			gui.State.Panels.Commits.SelectedLine--
		}
		return gui.handleGenericMergeCommandResult(err)
	})
}

func (gui *Gui) handleCommitEdit(g *gocui.Gui, v *gocui.View) error {
	applied, err := gui.handleMidRebaseCommand("edit")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	return gui.WithWaitingStatus(gui.Tr.SLocalize("RebasingStatus"), func() error {
		err = gui.GitCommand.InteractiveRebase(gui.State.Commits, gui.State.Panels.Commits.SelectedLine, "edit")
		return gui.handleGenericMergeCommandResult(err)
	})
}

func (gui *Gui) handleCommitAmendTo(g *gocui.Gui, v *gocui.View) error {
	return gui.createConfirmationPanel(gui.g, v, true, gui.Tr.SLocalize("AmendCommitTitle"), gui.Tr.SLocalize("AmendCommitPrompt"), func(*gocui.Gui, *gocui.View) error {
		return gui.WithWaitingStatus(gui.Tr.SLocalize("AmendingStatus"), func() error {
			err := gui.GitCommand.AmendTo(gui.State.Commits[gui.State.Panels.Commits.SelectedLine].Sha)
			return gui.handleGenericMergeCommandResult(err)
		})
	}, nil)
}

func (gui *Gui) handleCommitPick(g *gocui.Gui, v *gocui.View) error {
	applied, err := gui.handleMidRebaseCommand("pick")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	// at this point we aren't actually rebasing so we will interpret this as an
	// attempt to pull. We might revoke this later after enabling configurable keybindings
	return gui.handlePullFiles(g, v)
}

func (gui *Gui) handleCommitRevert(g *gocui.Gui, v *gocui.View) error {
	if err := gui.GitCommand.Revert(gui.State.Commits[gui.State.Panels.Commits.SelectedLine].Sha); err != nil {
		return gui.createErrorPanel(gui.g, err.Error())
	}
	gui.State.Panels.Commits.SelectedLine++
	return gui.refreshSidePanels(refreshOptions{mode: BLOCK_UI, scope: []int{COMMITS, BRANCHES}})
}

func (gui *Gui) handleCopyCommit(g *gocui.Gui, v *gocui.View) error {
	// get currently selected commit, add the sha to state.
	commit := gui.State.Commits[gui.State.Panels.Commits.SelectedLine]

	// we will un-copy it if it's already copied
	for index, cherryPickedCommit := range gui.State.CherryPickedCommits {
		if commit.Sha == cherryPickedCommit.Sha {
			gui.State.CherryPickedCommits = append(gui.State.CherryPickedCommits[0:index], gui.State.CherryPickedCommits[index+1:]...)
			return gui.renderBranchCommitsWithSelection()
		}
	}

	gui.addCommitToCherryPickedCommits(gui.State.Panels.Commits.SelectedLine)
	return gui.renderBranchCommitsWithSelection()
}

func (gui *Gui) cherryPickedCommitShaMap() map[string]bool {
	commitShaMap := map[string]bool{}
	for _, commit := range gui.State.CherryPickedCommits {
		commitShaMap[commit.Sha] = true
	}
	return commitShaMap
}

func (gui *Gui) addCommitToCherryPickedCommits(index int) {
	commitShaMap := gui.cherryPickedCommitShaMap()
	commitShaMap[gui.State.Commits[index].Sha] = true

	newCommits := []*commands.Commit{}
	for _, commit := range gui.State.Commits {
		if commitShaMap[commit.Sha] {
			// duplicating just the things we need to put in the rebase TODO list
			newCommits = append(newCommits, &commands.Commit{Name: commit.Name, Sha: commit.Sha})
		}
	}

	gui.State.CherryPickedCommits = newCommits
}

func (gui *Gui) handleCopyCommitRange(g *gocui.Gui, v *gocui.View) error {
	commitShaMap := gui.cherryPickedCommitShaMap()

	// find the last commit that is copied that's above our position
	// if there are none, startIndex = 0
	startIndex := 0
	for index, commit := range gui.State.Commits[0:gui.State.Panels.Commits.SelectedLine] {
		if commitShaMap[commit.Sha] {
			startIndex = index
		}
	}

	gui.Log.Info("commit copy start index: " + strconv.Itoa(startIndex))

	for index := startIndex; index <= gui.State.Panels.Commits.SelectedLine; index++ {
		gui.addCommitToCherryPickedCommits(index)
	}

	return gui.renderBranchCommitsWithSelection()
}

// HandlePasteCommits begins a cherry-pick rebase with the commits the user has copied
func (gui *Gui) HandlePasteCommits(g *gocui.Gui, v *gocui.View) error {
	return gui.createConfirmationPanel(g, v, true, gui.Tr.SLocalize("CherryPick"), gui.Tr.SLocalize("SureCherryPick"), func(g *gocui.Gui, v *gocui.View) error {
		return gui.WithWaitingStatus(gui.Tr.SLocalize("CherryPickingStatus"), func() error {
			err := gui.GitCommand.CherryPickCommits(gui.State.CherryPickedCommits)
			return gui.handleGenericMergeCommandResult(err)
		})
	}, nil)
}

func (gui *Gui) handleSwitchToCommitFilesPanel(g *gocui.Gui, v *gocui.View) error {
	if err := gui.refreshCommitFilesView(); err != nil {
		return err
	}

	return gui.switchFocus(g, gui.getCommitsView(), gui.getCommitFilesView())
}

func (gui *Gui) handleToggleDiffCommit(g *gocui.Gui, v *gocui.View) error {
	selectLimit := 2

	// get selected commit
	commit := gui.getSelectedCommit(g)
	if commit == nil {
		return gui.newStringTask("main", gui.Tr.SLocalize("NoCommitsThisBranch"))
	}

	// if already selected commit delete
	if idx, has := gui.hasCommit(gui.State.DiffEntries, commit.Sha); has {
		gui.State.DiffEntries = gui.unchooseCommit(gui.State.DiffEntries, idx)
	} else {
		if len(gui.State.DiffEntries) == 0 {
			gui.State.DiffEntries = []*commands.Commit{commit}
		} else {
			gui.State.DiffEntries = append(gui.State.DiffEntries[:1], commit)
		}
	}

	gui.setDiffMode()

	// if selected two commits, display diff between
	if len(gui.State.DiffEntries) == selectLimit {
		commitText, err := gui.GitCommand.DiffCommits(gui.State.DiffEntries[0].Sha, gui.State.DiffEntries[1].Sha)

		if err != nil {
			return gui.createErrorPanel(gui.g, err.Error())
		}

		gui.newStringTask("main", commitText)
	}
	return gui.renderBranchCommitsWithSelection()
}

func (gui *Gui) setDiffMode() {
	v := gui.getCommitsView()
	if len(gui.State.DiffEntries) != 0 {
		gui.State.Panels.Commits.SpecificDiffMode = true
		v.Title = gui.Tr.SLocalize("CommitsDiffTitle")
	} else {
		gui.State.Panels.Commits.SpecificDiffMode = false
		v.Title = gui.Tr.SLocalize("CommitsTitle")
	}
}

func (gui *Gui) hasCommit(commits []*commands.Commit, target string) (int, bool) {
	for idx, commit := range commits {
		if commit.Sha == target {
			return idx, true
		}
	}
	return -1, false
}

func (gui *Gui) unchooseCommit(commits []*commands.Commit, i int) []*commands.Commit {
	return append(commits[:i], commits[i+1:]...)
}

func (gui *Gui) handleCreateFixupCommit(g *gocui.Gui, v *gocui.View) error {
	commit := gui.getSelectedCommit(g)
	if commit == nil {
		return nil
	}

	return gui.createConfirmationPanel(g, v, true, gui.Tr.SLocalize("CreateFixupCommit"), gui.Tr.TemplateLocalize(
		"SureCreateFixupCommit",
		Teml{
			"commit": commit.Sha,
		},
	), func(g *gocui.Gui, v *gocui.View) error {
		if err := gui.GitCommand.CreateFixupCommit(commit.Sha); err != nil {
			return gui.createErrorPanel(g, err.Error())
		}

		return gui.refreshSidePanels(refreshOptions{mode: ASYNC})
	}, nil)
}

func (gui *Gui) handleSquashAllAboveFixupCommits(g *gocui.Gui, v *gocui.View) error {
	commit := gui.getSelectedCommit(g)
	if commit == nil {
		return nil
	}

	return gui.createConfirmationPanel(g, v, true, gui.Tr.SLocalize("SquashAboveCommits"), gui.Tr.TemplateLocalize(
		"SureSquashAboveCommits",
		Teml{
			"commit": commit.Sha,
		},
	), func(g *gocui.Gui, v *gocui.View) error {
		return gui.WithWaitingStatus(gui.Tr.SLocalize("SquashingStatus"), func() error {
			err := gui.GitCommand.SquashAllAboveFixupCommits(commit.Sha)
			return gui.handleGenericMergeCommandResult(err)
		})
	}, nil)
}

func (gui *Gui) handleTagCommit(g *gocui.Gui, v *gocui.View) error {
	// TODO: bring up menu asking if you want to make a lightweight or annotated tag
	// if annotated, switch to a subprocess to create the message

	commit := gui.getSelectedCommit(g)
	if commit == nil {
		return nil
	}

	return gui.handleCreateLightweightTag(commit.Sha)
}

func (gui *Gui) handleCreateLightweightTag(commitSha string) error {
	return gui.createPromptPanel(gui.g, gui.getCommitsView(), gui.Tr.SLocalize("TagNameTitle"), "", func(g *gocui.Gui, v *gocui.View) error {
		if err := gui.GitCommand.CreateLightweightTag(v.Buffer(), commitSha); err != nil {
			return gui.createErrorPanel(g, err.Error())
		}
		return gui.refreshSidePanels(refreshOptions{mode: ASYNC, scope: []int{COMMITS, TAGS}})
	})
}

func (gui *Gui) handleCheckoutCommit(g *gocui.Gui, v *gocui.View) error {
	commit := gui.getSelectedCommit(g)
	if commit == nil {
		return nil
	}

	return gui.createConfirmationPanel(g, gui.getCommitsView(), true, gui.Tr.SLocalize("checkoutCommit"), gui.Tr.SLocalize("SureCheckoutThisCommit"), func(g *gocui.Gui, v *gocui.View) error {
		return gui.handleCheckoutRef(commit.Sha, handleCheckoutRefOptions{})
	}, nil)
}

func (gui *Gui) renderBranchCommitsWithSelection() error {
	commitsView := gui.getCommitsView()

	gui.refreshSelectedLine(&gui.State.Panels.Commits.SelectedLine, len(gui.State.Commits))
	displayStrings := presentation.GetCommitListDisplayStrings(gui.State.Commits, gui.State.ScreenMode != SCREEN_NORMAL, gui.cherryPickedCommitShaMap(), gui.State.DiffEntries)
	gui.renderDisplayStrings(commitsView, displayStrings)
	if gui.g.CurrentView() == commitsView && commitsView.Context == "branch-commits" {
		if err := gui.handleCommitSelect(gui.g, commitsView); err != nil {
			return err
		}
	}

	return nil
}

func (gui *Gui) onCommitsTabClick(tabIndex int) error {
	contexts := []string{"branch-commits", "reflog-commits"}
	commitsView := gui.getCommitsView()
	commitsView.TabIndex = tabIndex

	return gui.switchCommitsPanelContext(contexts[tabIndex])
}

func (gui *Gui) switchCommitsPanelContext(context string) error {
	commitsView := gui.getCommitsView()
	commitsView.Context = context
	if err := gui.onSearchEscape(); err != nil {
		return err
	}

	contextTabIndexMap := map[string]int{
		"branch-commits": 0,
		"reflog-commits": 1,
	}

	commitsView.TabIndex = contextTabIndexMap[context]

	return gui.refreshCommitsViewWithSelection()
}

func (gui *Gui) refreshCommitsViewWithSelection() error {
	commitsView := gui.getCommitsView()

	switch commitsView.Context {
	case "branch-commits":
		return gui.renderBranchCommitsWithSelection()
	case "reflog-commits":
		return gui.renderReflogCommitsWithSelection()
	}

	return nil
}

func (gui *Gui) handleNextCommitsTab(g *gocui.Gui, v *gocui.View) error {
	return gui.onCommitsTabClick(
		utils.ModuloWithWrap(v.TabIndex+1, len(v.Tabs)),
	)
}

func (gui *Gui) handlePrevCommitsTab(g *gocui.Gui, v *gocui.View) error {
	return gui.onCommitsTabClick(
		utils.ModuloWithWrap(v.TabIndex-1, len(v.Tabs)),
	)
}

func (gui *Gui) handleCreateCommitResetMenu(g *gocui.Gui, v *gocui.View) error {
	commit := gui.getSelectedCommit(g)
	if commit == nil {
		return gui.createErrorPanel(gui.g, gui.Tr.SLocalize("NoCommitsThisBranch"))
	}

	return gui.createResetMenu(commit.Sha)
}

func (gui *Gui) onCommitsPanelSearchSelect(selectedLine int) error {
	commitsView := gui.getCommitsView()
	switch commitsView.Context {
	case "branch-commits":
		gui.State.Panels.Commits.SelectedLine = selectedLine
		return gui.handleCommitSelect(gui.g, commitsView)
	case "reflog-commits":
		gui.State.Panels.ReflogCommits.SelectedLine = selectedLine
		return gui.handleReflogCommitSelect(gui.g, commitsView)
	}
	return nil
}

func (gui *Gui) handleOpenSearchForCommitsPanel(g *gocui.Gui, v *gocui.View) error {
	// we usually lazyload these commits but now that we're searching we need to load them now
	if gui.State.Panels.Commits.LimitCommits {
		gui.State.Panels.Commits.LimitCommits = false
		if err := gui.refreshSidePanels(refreshOptions{mode: ASYNC, scope: []int{COMMITS}}); err != nil {
			return err
		}
	}

	return gui.handleOpenSearch(gui.g, v)
}

func (gui *Gui) handleResetCherryPick(g *gocui.Gui, v *gocui.View) error {
	gui.State.CherryPickedCommits = []*commands.Commit{}
	return gui.renderBranchCommitsWithSelection()
}
