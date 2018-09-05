package gui

import (

	// "io"
	// "io/ioutil"

	// "strings"

	"strings"

	"github.com/fatih/color"
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/commands"
	"strconv"
)

func (gui *Gui) stagedFiles() []commands.File {
	files := gui.State.Files
	result := make([]commands.File, 0)
	for _, file := range files {
		if file.HasStagedChanges {
			result = append(result, file)
		}
	}
	return result
}

func (gui *Gui) trackedFiles() []commands.File {
	files := gui.State.Files
	result := make([]commands.File, 0)
	for _, file := range files {
		if file.Tracked {
			result = append(result, file)
		}
	}
	return result
}

func (gui *Gui) stageSelectedFile(g *gocui.Gui) error {
	file, err := gui.getSelectedFile(g)
	if err != nil {
		return err
	}
	return gui.GitCommand.StageFile(file.Name)
}

func (gui *Gui) handleFilePress(g *gocui.Gui, v *gocui.View) error {
	file, err := gui.getSelectedFile(g)
	if err != nil {
		if err == gui.Errors.ErrNoFiles {
			return nil
		}
		return err
	}

	if file.HasMergeConflicts {
		return gui.handleSwitchToMerge(g, v)
	}

	if file.HasUnstagedChanges {
		gui.GitCommand.StageFile(file.Name)
	} else {
		gui.GitCommand.UnStageFile(file.Name, file.Tracked)
	}

	if err := gui.refreshFiles(g); err != nil {
		return err
	}

	return gui.handleFileSelect(g, v)
}

func (gui *Gui) allFilesStaged() bool {
	for _, file := range gui.State.Files {
		if file.HasUnstagedChanges {
			return false
		}
	}
	return true
}

func (gui *Gui) handleStageAll(g *gocui.Gui, v *gocui.View) error {
	var err error
	if gui.allFilesStaged() {
		err = gui.GitCommand.UnstageAll()
	} else {
		err = gui.GitCommand.StageAll()
	}
	if err != nil {
		_ = gui.createErrorPanel(g, err.Error())
	}

	if err := gui.refreshFiles(g); err != nil {
		return err
	}

	return gui.handleFileSelect(g, v)
}

func (gui *Gui) handleAddPatch(g *gocui.Gui, v *gocui.View) error {
	file, err := gui.getSelectedFile(g)
	if err != nil {
		if err == gui.Errors.ErrNoFiles {
			return nil
		}
		return err
	}
	if !file.HasUnstagedChanges {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("FileHasNoUnstagedChanges"))
	}
	if !file.Tracked {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("CannotGitAdd"))
	}

	gui.SubProcess = gui.GitCommand.AddPatch(file.Name)
	return gui.Errors.ErrSubProcess
}

func (gui *Gui) getSelectedFile(g *gocui.Gui) (commands.File, error) {
	if len(gui.State.Files) == 0 {
		return commands.File{}, gui.Errors.ErrNoFiles
	}
	filesView, err := g.View("files")
	if err != nil {
		panic(err)
	}
	lineNumber := gui.getItemPosition(filesView)
	return gui.State.Files[lineNumber], nil
}

func (gui *Gui) handleFileRemove(g *gocui.Gui, v *gocui.View) error {
	file, err := gui.getSelectedFile(g)
	if err != nil {
		if err == gui.Errors.ErrNoFiles {
			return nil
		}
		return err
	}
	var deleteVerb string
	if file.Tracked {
		deleteVerb = gui.Tr.SLocalize("checkout")
	} else {
		deleteVerb = gui.Tr.SLocalize("delete")
	}
	message := gui.Tr.TemplateLocalize(
		"SureTo",
		Teml{
			"deleteVerb": deleteVerb,
			"fileName":   file.Name,
		},
	)
	return gui.createConfirmationPanel(g, v, strings.Title(deleteVerb)+" file", message, func(g *gocui.Gui, v *gocui.View) error {
		if err := gui.GitCommand.RemoveFile(file); err != nil {
			return err
		}
		return gui.refreshFiles(g)
	}, nil)
}

func (gui *Gui) handleIgnoreFile(g *gocui.Gui, v *gocui.View) error {
	file, err := gui.getSelectedFile(g)
	if err != nil {
		return gui.createErrorPanel(g, err.Error())
	}
	if file.Tracked {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("CantIgnoreTrackFiles"))
	}
	if err := gui.GitCommand.Ignore(file.Name); err != nil {
		return gui.createErrorPanel(g, err.Error())
	}
	return gui.refreshFiles(g)
}

func (gui *Gui) renderfilesOptions(g *gocui.Gui, file *commands.File) error {
	optionsMap := map[string]string{
		"← → ↑ ↓":   gui.Tr.SLocalize("navigate"),
		"S":         gui.Tr.SLocalize("stashFiles"),
		"c":         gui.Tr.SLocalize("CommitChanges"),
		"o":         gui.Tr.SLocalize("open"),
		"i":         gui.Tr.SLocalize("ignore"),
		"d":         gui.Tr.SLocalize("delete"),
		"space":     gui.Tr.SLocalize("toggleStaged"),
		"R":         gui.Tr.SLocalize("refresh"),
		"t":         gui.Tr.SLocalize("addPatch"),
		"e":         gui.Tr.SLocalize("edit"),
		"a":         gui.Tr.SLocalize("toggleStagedAll"),
		"PgUp/PgDn": gui.Tr.SLocalize("scroll"),
	}
	if gui.State.HasMergeConflicts {
		optionsMap["a"] = gui.Tr.SLocalize("abortMerge")
		optionsMap["m"] = gui.Tr.SLocalize("resolveMergeConflicts")
	}
	if file == nil {
		return gui.renderOptionsMap(g, optionsMap)
	}
	if file.Tracked {
		optionsMap["d"] = gui.Tr.SLocalize("checkout")
	}
	return gui.renderOptionsMap(g, optionsMap)
}

func (gui *Gui) handleFileSelect(g *gocui.Gui, v *gocui.View) error {
	file, err := gui.getSelectedFile(g)
	if err != nil {
		if err != gui.Errors.ErrNoFiles {
			return err
		}
		gui.renderString(g, "main", gui.Tr.SLocalize("NoChangedFiles"))
		return gui.renderfilesOptions(g, nil)
	}
	gui.renderfilesOptions(g, &file)
	var content string
	if file.HasMergeConflicts {
		return gui.refreshMergePanel(g)
	}

	content = gui.GitCommand.Diff(file)
	return gui.renderString(g, "main", content)
}

func (gui *Gui) simpleEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	case key == gocui.KeyDelete:
		v.EditDelete(false)
	case key == gocui.KeyArrowDown:
		v.MoveCursor(0, 1, false)
	case key == gocui.KeyArrowUp:
		v.MoveCursor(0, -1, false)
	case key == gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0, false)
	case key == gocui.KeyArrowRight:
		v.MoveCursor(1, 0, false)
	case key == gocui.KeyTab:
		v.EditNewLine()
	case key == gocui.KeySpace:
		v.EditWrite(' ')
	case key == gocui.KeyInsert:
		v.Overwrite = !v.Overwrite
	default:
		v.EditWrite(ch)
	}

	gui.renderCommitCount(v)
}

func (gui *Gui) getCommitCount(view *gocui.View) int {
	return strings.Count(view.Buffer(), "") - 1
}

func (gui *Gui) renderCommitCount(view *gocui.View) error {
	num := 0
	offset := 5
	count := gui.getCommitCount(view)
	_, y0, x1, _ := gui.getConfirmationPanelDimensions(gui.g, view.Buffer())

	if count > 99 {
		num = 3
	} else if count > 9 {
		num = 2
	} else {
		num = 1
	}

	if _, err := gui.g.SetView("commitMessageCount", x1-num-offset, y0-1, x1-offset+1, y0+1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
	}

	return gui.renderString(gui.g, "commitMessageCount", strconv.Itoa(count))
}

func (gui *Gui) handleCommitPress(g *gocui.Gui, filesView *gocui.View) error {
	if len(gui.stagedFiles()) == 0 && !gui.State.HasMergeConflicts {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("NoStagedFilesToCommit"))
	}
	commitMessageView := gui.getCommitMessageView(g)
	g.Update(func(g *gocui.Gui) error {
		g.SetViewOnTop("commitMessage")
		g.SetViewOnTop("commitMessageCount")
		gui.switchFocus(g, filesView, commitMessageView)
		gui.renderCommitCount(commitMessageView)
		return nil
	})
	return nil
}

// handleCommitEditorPress - handle when the user wants to commit changes via
// their editor rather than via the popup panel
func (gui *Gui) handleCommitEditorPress(g *gocui.Gui, filesView *gocui.View) error {
	if len(gui.stagedFiles()) == 0 && !gui.State.HasMergeConflicts {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("NoStagedFilesToCommit"))
	}
	gui.PrepareSubProcess(g, "git", "commit")
	return nil
}

// PrepareSubProcess - prepare a subprocess for execution and tell the gui to switch to it
func (gui *Gui) PrepareSubProcess(g *gocui.Gui, commands ...string) {
	gui.SubProcess = gui.GitCommand.PrepareCommitSubProcess()
	g.Update(func(g *gocui.Gui) error {
		return gui.Errors.ErrSubProcess
	})
}

func (gui *Gui) editFile(filename string) error {
	sub, err := gui.OSCommand.EditFile(filename)
	if err != nil {
		return gui.createErrorPanel(gui.g, err.Error())
	}
	if sub != nil {
		gui.SubProcess = sub
		return gui.Errors.ErrSubProcess
	}
	return nil
}

func (gui *Gui) handleFileEdit(g *gocui.Gui, v *gocui.View) error {
	file, err := gui.getSelectedFile(g)
	if err != nil {
		return err
	}

	return gui.editFile(file.Name)
}

func (gui *Gui) handleFileOpen(g *gocui.Gui, v *gocui.View) error {
	file, err := gui.getSelectedFile(g)
	if err != nil {
		return err
	}
	return gui.openFile(file.Name)
}

func (gui *Gui) handleRefreshFiles(g *gocui.Gui, v *gocui.View) error {
	return gui.refreshFiles(g)
}

func (gui *Gui) refreshStateFiles() {
	// get files to stage
	files := gui.GitCommand.GetStatusFiles()
	gui.State.Files = gui.GitCommand.MergeStatusFiles(gui.State.Files, files)
	gui.updateHasMergeConflictStatus()
}

func (gui *Gui) updateHasMergeConflictStatus() error {
	merging, err := gui.GitCommand.IsInMergeState()
	if err != nil {
		return err
	}
	gui.State.HasMergeConflicts = merging
	return nil
}

func (gui *Gui) renderFile(file commands.File, filesView *gocui.View) {
	// potentially inefficient to be instantiating these color
	// objects with each render
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)
	if !file.Tracked && !file.HasStagedChanges {
		red.Fprintln(filesView, file.DisplayString)
		return
	}
	green.Fprint(filesView, file.DisplayString[0:1])
	red.Fprint(filesView, file.DisplayString[1:3])
	if file.HasUnstagedChanges {
		red.Fprintln(filesView, file.Name)
	} else {
		green.Fprintln(filesView, file.Name)
	}
}

func (gui *Gui) catSelectedFile(g *gocui.Gui) (string, error) {
	item, err := gui.getSelectedFile(g)
	if err != nil {
		if err != gui.Errors.ErrNoFiles {
			return "", err
		}
		return "", gui.renderString(g, "main", gui.Tr.SLocalize("NoFilesDisplay"))
	}
	if item.Type != "file" {
		return "", gui.renderString(g, "main", gui.Tr.SLocalize("NotAFile"))
	}
	cat, err := gui.GitCommand.CatFile(item.Name)
	if err != nil {
		gui.Log.Error(err)
		return "", gui.renderString(g, "main", err.Error())
	}
	return cat, nil
}

func (gui *Gui) refreshFiles(g *gocui.Gui) error {
	filesView, err := g.View("files")
	if err != nil {
		return err
	}
	gui.refreshStateFiles()
	filesView.Clear()
	for _, file := range gui.State.Files {
		gui.renderFile(file, filesView)
	}
	gui.correctCursor(filesView)
	if filesView == g.CurrentView() {
		gui.handleFileSelect(g, filesView)
	}
	return nil
}

func (gui *Gui) pullFiles(g *gocui.Gui, v *gocui.View) error {
	gui.createMessagePanel(g, v, "", gui.Tr.SLocalize("PullWait"))
	go func() {
		if err := gui.GitCommand.Pull(); err != nil {
			gui.createErrorPanel(g, err.Error())
		} else {
			gui.closeConfirmationPrompt(g)
			gui.refreshCommits(g)
			gui.refreshStatus(g)
		}
		gui.refreshFiles(g)
	}()
	return nil
}

func (gui *Gui) pushWithForceFlag(currentView *gocui.View, force bool) error {
	if err := gui.createMessagePanel(gui.g, currentView, "", gui.Tr.SLocalize("PushWait")); err != nil {
		return err
	}
	go func() {
		branchName := gui.State.Branches[0].Name
		if err := gui.GitCommand.Push(branchName, force); err != nil {
			_ = gui.createErrorPanel(gui.g, err.Error())
		} else {
			_ = gui.closeConfirmationPrompt(gui.g)
			_ = gui.refreshCommits(gui.g)
			_ = gui.refreshStatus(gui.g)
		}
	}()
	return nil
}

func (gui *Gui) pushFiles(g *gocui.Gui, v *gocui.View) error {
	// if we have pullables we'll ask if the user wants to force push
	_, pullables := gui.GitCommand.UpstreamDifferenceCount()
	if pullables == "?" || pullables == "0" {
		return gui.pushWithForceFlag(v, false)
	}
	err := gui.createConfirmationPanel(g, nil, gui.Tr.SLocalize("ForcePush"), gui.Tr.SLocalize("ForcePushPrompt"), func(g *gocui.Gui, v *gocui.View) error {
		return gui.pushWithForceFlag(v, true)
	}, nil)
	return err
}

func (gui *Gui) handleSwitchToMerge(g *gocui.Gui, v *gocui.View) error {
	mergeView, err := g.View("main")
	if err != nil {
		return err
	}
	file, err := gui.getSelectedFile(g)
	if err != nil {
		if err != gui.Errors.ErrNoFiles {
			return err
		}
		return nil
	}
	if !file.HasMergeConflicts {
		return gui.createErrorPanel(g, gui.Tr.SLocalize("FileNoMergeCons"))
	}
	gui.switchFocus(g, v, mergeView)
	return gui.refreshMergePanel(g)
}

func (gui *Gui) handleAbortMerge(g *gocui.Gui, v *gocui.View) error {
	if err := gui.GitCommand.AbortMerge(); err != nil {
		return gui.createErrorPanel(g, err.Error())
	}
	gui.createMessagePanel(g, v, "", gui.Tr.SLocalize("MergeAborted"))
	gui.refreshStatus(g)
	return gui.refreshFiles(g)
}

func (gui *Gui) handleResetHard(g *gocui.Gui, v *gocui.View) error {
	return gui.createConfirmationPanel(g, v, gui.Tr.SLocalize("ClearFilePanel"), gui.Tr.SLocalize("SureResetHardHead"), func(g *gocui.Gui, v *gocui.View) error {
		if err := gui.GitCommand.ResetHard(); err != nil {
			gui.createErrorPanel(g, err.Error())
		}
		return gui.refreshFiles(g)
	}, nil)
}

func (gui *Gui) openFile(filename string) error {
	if err := gui.OSCommand.OpenFile(filename); err != nil {
		return gui.createErrorPanel(gui.g, err.Error())
	}
	return nil
}
