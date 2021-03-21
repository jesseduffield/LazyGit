package gui

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation"
	"github.com/sirupsen/logrus"
)

const EXPANDED_ARROW = "▼"
const COLLAPSED_ARROW = "►"

type StatusLineManager struct {
	Files          []*models.File
	Tree           *models.StatusLineNode
	TreeMode       bool
	Log            *logrus.Entry
	CollapsedPaths map[string]bool
}

func NewStatusLineManager(files []*models.File, log *logrus.Entry) *StatusLineManager {
	return &StatusLineManager{
		Files:          files,
		Log:            log,
		TreeMode:       true, // always true for now
		CollapsedPaths: map[string]bool{},
	}
}

func (m *StatusLineManager) GetItemAtIndex(index int) *models.StatusLineNode {
	if m.TreeMode {
		// need to traverse the three depth first until we get to the index.
		return m.Tree.GetNodeAtIndex(index+1, m.CollapsedPaths) // ignoring root
	}

	m.Log.Warn(index)
	if index > len(m.Files)-1 {
		return nil
	}

	return &models.StatusLineNode{File: m.Files[index]}
}

func (m *StatusLineManager) GetAllItems() []*models.StatusLineNode {
	return m.Tree.Flatten(m.CollapsedPaths)[1:] // ignoring root
}

func (m *StatusLineManager) GetItemsLength() int {
	return m.Tree.Size(m.CollapsedPaths) - 1 // ignoring root
}

func (m *StatusLineManager) GetAllFiles() []*models.File {
	return m.Files
}

func (m *StatusLineManager) SetFiles(files []*models.File) {
	m.Files = files
	m.Tree = GetTreeFromStatusFiles(files)
}

func (m *StatusLineManager) Render(diffName string, submoduleConfigs []*models.SubmoduleConfig) []string {
	return m.renderAux(m.Tree, "", -1, diffName, submoduleConfigs)
}

const INNER_ITEM = "├─ "
const LAST_ITEM = "└─ "
const NESTED = "│  "
const NOTHING = "   "

func (m *StatusLineManager) IsCollapsed(s *models.StatusLineNode) bool {
	return m.CollapsedPaths[s.GetPath()]
}

func (m *StatusLineManager) ToggleCollapsed(s *models.StatusLineNode) {
	m.CollapsedPaths[s.GetPath()] = !m.CollapsedPaths[s.GetPath()]
}

func (m *StatusLineManager) renderAux(s *models.StatusLineNode, prefix string, depth int, diffName string, submoduleConfigs []*models.SubmoduleConfig) []string {
	isRoot := depth == -1
	if s == nil {
		return []string{}
	}

	getLine := func() string {
		return prefix + presentation.GetStatusNodeLine(s.GetHasUnstagedChanges(), s.GetHasStagedChanges(), s.Name, diffName, submoduleConfigs, s.File)
	}

	if s.IsLeaf() {
		if isRoot {
			return []string{}
		}
		return []string{getLine()}
	}

	if m.IsCollapsed(s) {
		return []string{fmt.Sprintf("%s %s", getLine(), COLLAPSED_ARROW)}
	}

	arr := []string{}
	if !isRoot {
		arr = append(arr, fmt.Sprintf("%s %s", getLine(), EXPANDED_ARROW))
	}

	newPrefix := prefix
	if strings.HasSuffix(prefix, LAST_ITEM) {
		newPrefix = strings.TrimSuffix(prefix, LAST_ITEM) + NOTHING
	} else if strings.HasSuffix(prefix, INNER_ITEM) {
		newPrefix = strings.TrimSuffix(prefix, INNER_ITEM) + NESTED
	}

	for i, child := range s.Children {
		isLast := i == len(s.Children)-1

		var childPrefix string
		if isRoot {
			childPrefix = newPrefix
		} else if isLast {
			childPrefix = newPrefix + LAST_ITEM
		} else {
			childPrefix = newPrefix + INNER_ITEM
		}

		arr = append(arr, m.renderAux(child, childPrefix, depth+1, diffName, submoduleConfigs)...)
	}

	return arr
}
