package gui

import (
	"os"

	"github.com/jesseduffield/gocui"
)

// when a user runs lazygit with the LAZYGIT_NEW_DIR_FILE env variable defined
// we will write the current directory to that file on exit so that their
// shell can then change to that directory. That means you don't get kicked
// back to the directory that you started with.
func (gui *Gui) recordCurrentDirectory() error {
	if os.Getenv("LAZYGIT_NEW_DIR_FILE") == "" {
		return nil
	}

	// determine current directory, set it in LAZYGIT_NEW_DIR_FILE
	dirName, err := os.Getwd()
	if err != nil {
		return err
	}

	return gui.OSCommand.CreateFileWithContent(os.Getenv("LAZYGIT_NEW_DIR_FILE"), dirName)
}

func (gui *Gui) handleQuitWithoutChangingDirectory(g *gocui.Gui, v *gocui.View) error {
	gui.State.RetainOriginalDir = true
	return gui.quit()
}

func (gui *Gui) handleQuit() error {
	gui.State.RetainOriginalDir = false
	return gui.quit()
}

func (gui *Gui) handleTopLevelReturn(g *gocui.Gui, v *gocui.View) error {
	if gui.inDiffMode() {
		return gui.exitDiffMode()
	}
	if gui.inFilterMode() {
		return gui.exitFilterMode()
	}
	if gui.isExtensiveView(v) {
		return gui.handleCloseExtensiveView(g, v)
	}

	if gui.Config.GetUserConfig().GetBool("quitOnTopLevelReturn") {
		return gui.handleQuit()
	}

	return nil
}

func (gui *Gui) quit() error {
	if gui.State.Updating {
		return gui.createUpdateQuitConfirmation()
	}

	if gui.Config.GetUserConfig().GetBool("confirmOnQuit") {
		return gui.ask(askOpts{
			returnToView:       gui.g.CurrentView(),
			returnFocusOnClose: true,
			title:              "",
			prompt:             gui.Tr.SLocalize("ConfirmQuit"),
			handleConfirm: func() error {
				return gocui.ErrQuit
			},
		})
	}

	return gocui.ErrQuit
}
