package gui

import (
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/commands"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation"
)

// list panel functions

func (gui *Gui) getSelectedStashEntry() *commands.StashEntry {
	selectedLine := gui.State.Panels.Stash.SelectedLine
	if selectedLine == -1 {
		return nil
	}

	return gui.State.StashEntries[selectedLine]
}

func (gui *Gui) handleStashEntrySelect(g *gocui.Gui, v *gocui.View) error {
	if gui.popupPanelFocused() {
		return nil
	}

	gui.State.SplitMainPanel = false

	if _, err := gui.g.SetCurrentView(v.Name()); err != nil {
		return err
	}

	gui.getMainView().Title = "Stash"

	stashEntry := gui.getSelectedStashEntry()
	if stashEntry == nil {
		return gui.newStringTask("main", gui.Tr.SLocalize("NoStashEntries"))
	}
	v.FocusPoint(0, gui.State.Panels.Stash.SelectedLine)

	if gui.inDiffMode() {
		return gui.renderDiff()
	}

	cmd := gui.OSCommand.ExecutableFromString(
		gui.GitCommand.ShowStashEntryCmdStr(stashEntry.Index),
	)
	if err := gui.newPtyTask("main", cmd); err != nil {
		gui.Log.Error(err)
	}

	return nil
}

func (gui *Gui) refreshStashEntries(g *gocui.Gui) error {
	gui.State.StashEntries = gui.GitCommand.GetStashEntries(gui.State.FilterPath)

	gui.refreshSelectedLine(&gui.State.Panels.Stash.SelectedLine, len(gui.State.StashEntries))

	stashView := gui.getStashView()

	displayStrings := presentation.GetStashEntryListDisplayStrings(gui.State.StashEntries, gui.State.Diff.Ref)
	gui.renderDisplayStrings(stashView, displayStrings)

	return gui.resetOrigin(stashView)
}

// specific functions

func (gui *Gui) handleStashApply(g *gocui.Gui, v *gocui.View) error {
	skipStashWarning := gui.Config.GetUserConfig().GetBool("gui.skipStashWarning")

	apply := func() error {
		return gui.stashDo(g, v, "apply")
	}

	if skipStashWarning {
		return apply()
	}

	title := gui.Tr.SLocalize("StashApply")
	message := gui.Tr.SLocalize("SureApplyStashEntry")
	return gui.createConfirmationPanel(g, v, true, title, message, func(g *gocui.Gui, v *gocui.View) error {
		return apply()
	}, nil)
}

func (gui *Gui) handleStashPop(g *gocui.Gui, v *gocui.View) error {
	skipStashWarning := gui.Config.GetUserConfig().GetBool("gui.skipStashWarning")

	pop := func() error {
		return gui.stashDo(g, v, "pop")
	}

	if skipStashWarning {
		return pop()
	}

	title := gui.Tr.SLocalize("StashPop")
	message := gui.Tr.SLocalize("SurePopStashEntry")
	return gui.createConfirmationPanel(g, v, true, title, message, func(g *gocui.Gui, v *gocui.View) error {
		return pop()
	}, nil)
}

func (gui *Gui) handleStashDrop(g *gocui.Gui, v *gocui.View) error {
	title := gui.Tr.SLocalize("StashDrop")
	message := gui.Tr.SLocalize("SureDropStashEntry")
	return gui.createConfirmationPanel(g, v, true, title, message, func(g *gocui.Gui, v *gocui.View) error {
		return gui.stashDo(g, v, "drop")
	}, nil)
}

func (gui *Gui) stashDo(g *gocui.Gui, v *gocui.View, method string) error {
	stashEntry := gui.getSelectedStashEntry()
	if stashEntry == nil {
		errorMessage := gui.Tr.TemplateLocalize(
			"NoStashTo",
			Teml{
				"method": method,
			},
		)
		return gui.createErrorPanel(errorMessage)
	}
	if err := gui.GitCommand.StashDo(stashEntry.Index, method); err != nil {
		return gui.surfaceError(err)
	}
	return gui.refreshSidePanels(refreshOptions{scope: []int{STASH, FILES}})
}

func (gui *Gui) handleStashSave(stashFunc func(message string) error) error {
	if len(gui.trackedFiles()) == 0 && len(gui.stagedFiles()) == 0 {
		return gui.createErrorPanel(gui.Tr.SLocalize("NoTrackedStagedFilesStash"))
	}
	return gui.createPromptPanel(gui.g, gui.getFilesView(), gui.Tr.SLocalize("StashChanges"), "", func(g *gocui.Gui, v *gocui.View) error {
		if err := stashFunc(gui.trimmedContent(v)); err != nil {
			return gui.surfaceError(err)
		}
		return gui.refreshSidePanels(refreshOptions{scope: []int{STASH, FILES}})
	})
}

func (gui *Gui) onStashPanelSearchSelect(selectedLine int) error {
	gui.State.Panels.Stash.SelectedLine = selectedLine
	return gui.handleStashEntrySelect(gui.g, gui.getStashView())
}
