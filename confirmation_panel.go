// lots of this has been directly ported from one of the example files, will brush up later

// Copyright 2014 The gocui Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"strings"

	"github.com/fatih/color"
	"github.com/jesseduffield/gocui"
)

func wrappedConfirmationFunction(function func(*gocui.Gui, *gocui.View) error) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		if function != nil {
			if err := function(g, v); err != nil {
				panic(err)
			}
		}
		return closeConfirmationPrompt(g)
	}
}

func closeConfirmationPrompt(g *gocui.Gui) error {
	view, err := g.View("confirmation")
	if err != nil {
		panic(err)
	}
	if err := returnFocus(g, view); err != nil {
		panic(err)
	}
	g.DeleteKeybindings("confirmation")
	return g.DeleteView("confirmation")
}

func getMessageHeight(message string, width int) int {
	lines := strings.Split(message, "\n")
	lineCount := 0
	for _, line := range lines {
		lineCount += len(line)/width + 1
	}
	return lineCount
}

func getConfirmationPanelDimensions(g *gocui.Gui, prompt string) (int, int, int, int) {
	width, height := g.Size()
	panelWidth := 60
	panelHeight := getMessageHeight(prompt, panelWidth)
	return width/2 - panelWidth/2,
		height/2 - panelHeight/2 - panelHeight%2 - 1,
		width/2 + panelWidth/2,
		height/2 + panelHeight/2
}

func createPromptPanel(g *gocui.Gui, currentView *gocui.View, title string, initialValue *[]byte, handleYes func(*gocui.Gui, *gocui.View) error) error {
	// only need to fit one line
	x0, y0, x1, y1 := getConfirmationPanelDimensions(g, "")
	if confirmationView, err := g.SetView("confirmation", x0, y0, x1, y1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		g.Cursor = true

		confirmationView.Editable = true
		confirmationView.Title = title
		confirmationView.FgColor = gocui.ColorWhite
		confirmationView.Write(*initialValue)
		confirmationView.SetCursor(len(*initialValue), 0)
		switchFocus(g, currentView, confirmationView)
		return setKeyBindings(g, handleYes, func(gui *gocui.Gui, view *gocui.View) error {
			*initialValue = []byte(strings.TrimSpace(view.Buffer()))
			return nil
		})
	}
	return nil
}

func createConfirmationPanel(g *gocui.Gui, currentView *gocui.View, title, prompt string, handleYes, handleNo func(*gocui.Gui, *gocui.View) error) error {
	g.Update(func(g *gocui.Gui) error {
		// delete the existing confirmation panel if it exists
		if view, _ := g.View("confirmation"); view != nil {
			if err := closeConfirmationPrompt(g); err != nil {
				panic(err)
			}
		}
		x0, y0, x1, y1 := getConfirmationPanelDimensions(g, prompt)
		if confirmationView, err := g.SetView("confirmation", x0, y0, x1, y1, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			confirmationView.Title = title
			confirmationView.FgColor = gocui.ColorWhite
			renderString(g, "confirmation", prompt)
			switchFocus(g, currentView, confirmationView)
			return setKeyBindings(g, handleYes, handleNo)
		}
		return nil
	})
	return nil
}

func setKeyBindings(g *gocui.Gui, handleYes, handleNo func(*gocui.Gui, *gocui.View) error) error {
	renderString(g, "options", "esc: close, enter: confirm")
	if err := g.SetKeybinding("confirmation", gocui.KeyEnter, gocui.ModNone, wrappedConfirmationFunction(handleYes)); err != nil {
		return err
	}

	return g.SetKeybinding("confirmation", gocui.KeyEsc, gocui.ModNone, wrappedConfirmationFunction(handleNo))
}

func createMessagePanel(g *gocui.Gui, currentView *gocui.View, title, prompt string) error {
	return createConfirmationPanel(g, currentView, title, prompt, nil, nil)
}

func createErrorPanel(g *gocui.Gui, message string) error {
	currentView := g.CurrentView()
	colorFunction := color.New(color.FgRed).SprintFunc()
	coloredMessage := colorFunction(strings.TrimSpace(message))
	return createConfirmationPanel(g, currentView, "Error", coloredMessage, nil, nil)
}
