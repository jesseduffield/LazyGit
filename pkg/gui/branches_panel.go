package gui

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/commands"
	"github.com/jesseduffield/lazygit/pkg/git"
)

func (gui *Gui) handleBranchPress(g *gocui.Gui, v *gocui.View) error {

	index := gui.getItemPosition(v)
	if index == 0 {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("AlreadyCheckedOutBranch"))
	}

	branch := gui.getSelectedBranch(v)

	err := gui.GitCommand.Checkout(branch.Name, false)
	if err != nil {

		err = gui.createErrorPanel(g, err.Error())
		if err != nil {
			return err
		}

	}

	err = gui.refreshSidePanels(g)
	if err != nil {
		return err
	}

	return nil
}

func (gui *Gui) handleForceCheckout(g *gocui.Gui, v *gocui.View) error {

	branch := gui.getSelectedBranch(v)
	message := gui.Tr.SLocalize("SureForceCheckout")
	title := gui.Tr.SLocalize("ForceCheckoutBranch")

	err := gui.createConfirmationPanel(g, v, title, message,
		func(g *gocui.Gui, v *gocui.View) error {

			err := gui.GitCommand.Checkout(branch.Name, true)
			if err != nil {
				gui.createErrorPanel(g, err.Error())
			}

			err = gui.refreshSidePanels(g)
			if err != nil {
				return err
			}

			return nil
		}, nil)
	if err != nil {
		return err
	}

	return nil
}

func (gui *Gui) handleCheckoutByName(g *gocui.Gui, v *gocui.View) error {

	err := gui.createPromptPanel(g, v, gui.Tr.SLocalize("BranchName")+":",
		func(g *gocui.Gui, v *gocui.View) error {

			err := gui.GitCommand.Checkout(gui.trimmedContent(v), false)
			if err != nil {
				return gui.createErrorPanel(g, err.Error())
			}

			err = gui.refreshSidePanels(g)
			if err != nil {
				return err
			}

			return nil
		})
	if err != nil {
		return err
	}

	return nil
}

func (gui *Gui) handleNewBranch(g *gocui.Gui, v *gocui.View) error {

	branch := gui.State.Branches[0]
	message := gui.Tr.TemplateLocalize(
		"NewBranchNameBranchOff",
		Teml{
			"branchName": branch.Name,
		},
	)

	err := gui.createPromptPanel(g, v, message,
		func(g *gocui.Gui, v *gocui.View) error {

			err := gui.GitCommand.NewBranch(gui.trimmedContent(v))
			if err != nil {
				return gui.createErrorPanel(g, err.Error())
			}

			err = gui.refreshSidePanels(g)
			if err != nil {
				return err
			}

			err = gui.handleBranchSelect(g, v)
			if err != nil {
				return err
			}

			return nil
		})
	if err != nil {
		return err
	}

	return nil
}

func (gui *Gui) handleDeleteBranch(g *gocui.Gui, v *gocui.View) error {
	return gui.deleteBranch(g, v, false)
}

func (gui *Gui) handleForceDeleteBranch(g *gocui.Gui, v *gocui.View) error {
	return gui.deleteBranch(g, v, true)
}

func (gui *Gui) deleteBranch(g *gocui.Gui, v *gocui.View, force bool) error {

	var messageId string
	checkedOutBranch := gui.State.Branches[0]
	selectedBranch := gui.getSelectedBranch(v)
	title := gui.Tr.SLocalize("DeleteBranch")

	if force {
		messageId = "ForceDeleteBranchMessage"
	} else {
		messageId = "DeleteBranchMessage"
	}

	message := gui.Tr.TemplateLocalize(
		messageId,
		Teml{
			"selectedBranchName": selectedBranch.Name,
		},
	)

	if checkedOutBranch.Name == selectedBranch.Name {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("CantDeleteCheckOutBranch"))
	}

	return gui.createConfirmationPanel(g, v, title, message,
		func(g *gocui.Gui, v *gocui.View) error {

			err := gui.GitCommand.DeleteBranch(selectedBranch.Name, force)
			if err != nil {
				return gui.createErrorPanel(g, err.Error())
			}

			return gui.refreshSidePanels(g)
		}, nil)
}

func (gui *Gui) handleMerge(g *gocui.Gui, v *gocui.View) error {

	checkedOutBranch := gui.State.Branches[0]
	selectedBranch := gui.getSelectedBranch(v)

	defer gui.refreshSidePanels(g)

	if checkedOutBranch.Name == selectedBranch.Name {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("CantMergeBranchIntoItself"))
	}

	err := gui.GitCommand.Merge(selectedBranch.Name)
	if err != nil {
		return gui.createErrorPanel(g, err.Error())
	}

	return nil
}

func (gui *Gui) getSelectedBranch(v *gocui.View) commands.Branch {

	lineNumber := gui.getItemPosition(v)

	return gui.State.Branches[lineNumber]
}

func (gui *Gui) renderBranchesOptions(g *gocui.Gui) error {
	return gui.renderOptionsMap(g, map[string]string{
		"space":   gui.Tr.SLocalize("checkout"),
		"f":       gui.Tr.SLocalize("forceCheckout"),
		"m":       gui.Tr.SLocalize("merge"),
		"c":       gui.Tr.SLocalize("checkoutByName"),
		"n":       gui.Tr.SLocalize("newBranch"),
		"d":       gui.Tr.SLocalize("deleteBranch"),
		"D":       gui.Tr.SLocalize("forceDeleteBranch"),
		"← → ↑ ↓": gui.Tr.SLocalize("navigate"),
	})
}

// may want to standardise how these select methods work
func (gui *Gui) handleBranchSelect(g *gocui.Gui, v *gocui.View) error {

	err := gui.renderBranchesOptions(g)
	if err != nil {
		return err
	}

	// TODO This really shouldn't happen: there should always be a master branch
	if len(gui.State.Branches) == 0 {
		return gui.renderString(g, "main", gui.Tr.SLocalize("NoBranchesThisRepo"))
	}

	go func() error {

		branch := gui.getSelectedBranch(v)

		diff, err := gui.GitCommand.GetBranchGraph(branch.Name)

		if err != nil && strings.HasPrefix(diff, "fatal: ambiguous argument") {
			diff = gui.Tr.SLocalize("NoTrackingThisBranch")
		}

		err = gui.renderString(g, "main", diff)
		if err != nil {
			return err
		}

		return nil
	}()

	return nil
}

// gui.refreshStatus is called at the end of this because that's when we can
// be sure there is a state.Branches array to pick the current branch from
func (gui *Gui) refreshBranches(g *gocui.Gui) error {

	g.Update(
		func(g *gocui.Gui) error {

			v, err := g.View("branches")
			if err != nil {
				panic(err)
			}

			builder, err := git.NewBranchListBuilder(gui.Log, gui.GitCommand)
			if err != nil {
				return err
			}

			gui.State.Branches = builder.Build()

			v.Clear()

			for _, branch := range gui.State.Branches {
				fmt.Fprintln(v, branch.GetDisplayString())
			}

			err = gui.resetOrigin(v)
			if err != nil {
				return err
			}

			return gui.refreshStatus(g)
		})

	return nil
}
