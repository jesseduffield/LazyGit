package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jesseduffield/lazygit/pkg/i18n"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	gogit "gopkg.in/src-d/go-git.v4"
)

type fileInfoMock struct {
	name        string
	size        int64
	fileMode    os.FileMode
	fileModTime time.Time
	isDir       bool
	sys         interface{}
}

// Name is a function.
func (f fileInfoMock) Name() string {
	return f.name
}

// Size is a function.
func (f fileInfoMock) Size() int64 {
	return f.size
}

// Mode is a function.
func (f fileInfoMock) Mode() os.FileMode {
	return f.fileMode
}

// ModTime is a function.
func (f fileInfoMock) ModTime() time.Time {
	return f.fileModTime
}

// IsDir is a function.
func (f fileInfoMock) IsDir() bool {
	return f.isDir
}

// Sys is a function.
func (f fileInfoMock) Sys() interface{} {
	return f.sys
}

func newDummyLog() *logrus.Entry {
	log := logrus.New()
	log.Out = ioutil.Discard
	return log.WithField("test", "test")
}

func newDummyGitCommand() *GitCommand {
	return &GitCommand{
		Log:                newDummyLog(),
		OSCommand:          newDummyOSCommand(),
		Tr:                 i18n.NewLocalizer(newDummyLog()),
		getGlobalGitConfig: func(string) (string, error) { return "", nil },
		getLocalGitConfig:  func(string) (string, error) { return "", nil },
		removeFile:         func(string) error { return nil },
	}
}

// TestVerifyInGitRepo is a function.
func TestVerifyInGitRepo(t *testing.T) {
	type scenario struct {
		testName string
		runCmd   func(string) error
		test     func(error)
	}

	scenarios := []scenario{
		{
			"Valid git repository",
			func(string) error {
				return nil
			},
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"Not a valid git repository",
			func(string) error {
				return fmt.Errorf("fatal: Not a git repository (or any of the parent directories): .git")
			},
			func(err error) {
				assert.Error(t, err)
				assert.Regexp(t, "fatal: .ot a git repository \\(or any of the parent directories\\): \\.git", err.Error())
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			s.test(verifyInGitRepo(s.runCmd))
		})
	}
}

// TestNavigateToRepoRootDirectory is a function.
func TestNavigateToRepoRootDirectory(t *testing.T) {
	type scenario struct {
		testName string
		stat     func(string) (os.FileInfo, error)
		chdir    func(string) error
		test     func(error)
	}

	scenarios := []scenario{
		{
			"Navigate to git repository",
			func(string) (os.FileInfo, error) {
				return fileInfoMock{isDir: true}, nil
			},
			func(string) error {
				return nil
			},
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"An error occurred when getting path informations",
			func(string) (os.FileInfo, error) {
				return nil, fmt.Errorf("An error occurred")
			},
			func(string) error {
				return nil
			},
			func(err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "An error occurred")
			},
		},
		{
			"An error occurred when trying to move one path backward",
			func(string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			func(string) error {
				return fmt.Errorf("An error occurred")
			},
			func(err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "An error occurred")
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			s.test(navigateToRepoRootDirectory(s.stat, s.chdir))
		})
	}
}

// TestSetupRepositoryAndWorktree is a function.
func TestSetupRepositoryAndWorktree(t *testing.T) {
	type scenario struct {
		testName          string
		openGitRepository func(string) (*gogit.Repository, error)
		sLocalize         func(string) string
		test              func(*gogit.Repository, *gogit.Worktree, error)
	}

	scenarios := []scenario{
		{
			"A gitconfig parsing error occurred",
			func(string) (*gogit.Repository, error) {
				return nil, fmt.Errorf(`unquoted '\' must be followed by new line`)
			},
			func(string) string {
				return "error translated"
			},
			func(r *gogit.Repository, w *gogit.Worktree, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "error translated")
			},
		},
		{
			"A gogit error occurred",
			func(string) (*gogit.Repository, error) {
				return nil, fmt.Errorf("Error from inside gogit")
			},
			func(string) string { return "" },
			func(r *gogit.Repository, w *gogit.Worktree, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "Error from inside gogit")
			},
		},
		{
			"An error occurred cause git repository is a bare repository",
			func(string) (*gogit.Repository, error) {
				return &gogit.Repository{}, nil
			},
			func(string) string { return "" },
			func(r *gogit.Repository, w *gogit.Worktree, err error) {
				assert.Error(t, err)
				assert.Equal(t, gogit.ErrIsBareRepository, err)
			},
		},
		{
			"Setup done properly",
			func(string) (*gogit.Repository, error) {
				assert.NoError(t, os.RemoveAll("/tmp/lazygit-test"))
				r, err := gogit.PlainInit("/tmp/lazygit-test", false)
				assert.NoError(t, err)
				return r, nil
			},
			func(string) string { return "" },
			func(r *gogit.Repository, w *gogit.Worktree, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, w)
				assert.NotNil(t, r)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			s.test(setupRepositoryAndWorktree(s.openGitRepository, s.sLocalize))
		})
	}
}

// TestNewGitCommand is a function.
func TestNewGitCommand(t *testing.T) {
	actual, err := os.Getwd()
	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, os.Chdir(actual))
	}()

	type scenario struct {
		testName string
		setup    func()
		test     func(*GitCommand, error)
	}

	scenarios := []scenario{
		{
			"An error occurred, folder doesn't contains a git repository",
			func() {
				assert.NoError(t, os.Chdir("/tmp"))
			},
			func(gitCmd *GitCommand, err error) {
				assert.Error(t, err)
				assert.Regexp(t, "fatal: .ot a git repository \\(or any of the parent directories\\): \\.git", err.Error())
			},
		},
		{
			"New GitCommand object created",
			func() {
				assert.NoError(t, os.RemoveAll("/tmp/lazygit-test"))
				_, err := gogit.PlainInit("/tmp/lazygit-test", false)
				assert.NoError(t, err)
				assert.NoError(t, os.Chdir("/tmp/lazygit-test"))
			},
			func(gitCmd *GitCommand, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			s.setup()
			s.test(NewGitCommand(newDummyLog(), newDummyOSCommand(), i18n.NewLocalizer(newDummyLog())))
		})
	}
}

// TestGitCommandGetStashEntries is a function.
func TestGitCommandGetStashEntries(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func([]*StashEntry)
	}

	scenarios := []scenario{
		{
			"No stash entries found",
			func(string, ...string) *exec.Cmd {
				return exec.Command("echo")
			},
			func(entries []*StashEntry) {
				assert.Len(t, entries, 0)
			},
		},
		{
			"Several stash entries found",
			func(string, ...string) *exec.Cmd {
				return exec.Command("echo", "WIP on add-pkg-commands-test: 55c6af2 increase parallel build\nWIP on master: bb86a3f update github template")
			},
			func(entries []*StashEntry) {
				expected := []*StashEntry{
					{
						0,
						"WIP on add-pkg-commands-test: 55c6af2 increase parallel build",
						"WIP on add-pkg-commands-test: 55c6af2 increase parallel build",
					},
					{
						1,
						"WIP on master: bb86a3f update github template",
						"WIP on master: bb86a3f update github template",
					},
				}

				assert.Len(t, entries, 2)
				assert.EqualValues(t, expected, entries)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command

			s.test(gitCmd.GetStashEntries())
		})
	}
}

// TestGitCommandGetStashEntryDiff is a function.
func TestGitCommandGetStashEntryDiff(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"stash", "show", "-p", "--color", "stash@{1}"}, args)

		return exec.Command("echo")
	}

	_, err := gitCmd.GetStashEntryDiff(1)

	assert.NoError(t, err)
}

// TestGitCommandGetStatusFiles is a function.
func TestGitCommandGetStatusFiles(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func([]*File)
	}

	scenarios := []scenario{
		{
			"No files found",
			func(cmd string, args ...string) *exec.Cmd {
				return exec.Command("echo")
			},
			func(files []*File) {
				assert.Len(t, files, 0)
			},
		},
		{
			"Several files found",
			func(cmd string, args ...string) *exec.Cmd {
				return exec.Command(
					"echo",
					"MM file1.txt\nA  file3.txt\nAM file2.txt\n?? file4.txt",
				)
			},
			func(files []*File) {
				assert.Len(t, files, 4)

				expected := []*File{
					{
						Name:               "file1.txt",
						HasStagedChanges:   true,
						HasUnstagedChanges: true,
						Tracked:            true,
						Deleted:            false,
						HasMergeConflicts:  false,
						DisplayString:      "MM file1.txt",
						Type:               "other",
					},
					{
						Name:               "file3.txt",
						HasStagedChanges:   true,
						HasUnstagedChanges: false,
						Tracked:            false,
						Deleted:            false,
						HasMergeConflicts:  false,
						DisplayString:      "A  file3.txt",
						Type:               "other",
					},
					{
						Name:               "file2.txt",
						HasStagedChanges:   true,
						HasUnstagedChanges: true,
						Tracked:            false,
						Deleted:            false,
						HasMergeConflicts:  false,
						DisplayString:      "AM file2.txt",
						Type:               "other",
					},
					{
						Name:               "file4.txt",
						HasStagedChanges:   false,
						HasUnstagedChanges: true,
						Tracked:            false,
						Deleted:            false,
						HasMergeConflicts:  false,
						DisplayString:      "?? file4.txt",
						Type:               "other",
					},
				}

				assert.EqualValues(t, expected, files)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command

			s.test(gitCmd.GetStatusFiles())
		})
	}
}

// TestGitCommandStashDo is a function.
func TestGitCommandStashDo(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"stash", "drop", "stash@{1}"}, args)

		return exec.Command("echo")
	}

	assert.NoError(t, gitCmd.StashDo(1, "drop"))
}

// TestGitCommandStashSave is a function.
func TestGitCommandStashSave(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"stash", "save", "A stash message"}, args)

		return exec.Command("echo")
	}

	assert.NoError(t, gitCmd.StashSave("A stash message"))
}

// TestGitCommandCommitAmend is a function.
func TestGitCommandCommitAmend(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"commit", "--amend", "--allow-empty"}, args)

		return exec.Command("echo")
	}

	_, err := gitCmd.PrepareCommitAmendSubProcess().CombinedOutput()
	assert.NoError(t, err)
}

// TestGitCommandMergeStatusFiles is a function.
func TestGitCommandMergeStatusFiles(t *testing.T) {
	type scenario struct {
		testName string
		oldFiles []*File
		newFiles []*File
		test     func([]*File)
	}

	scenarios := []scenario{
		{
			"Old file and new file are the same",
			[]*File{},
			[]*File{
				{
					Name: "new_file.txt",
				},
			},
			func(files []*File) {
				expected := []*File{
					{
						Name: "new_file.txt",
					},
				}

				assert.Len(t, files, 1)
				assert.EqualValues(t, expected, files)
			},
		},
		{
			"Several files to merge, with some identical",
			[]*File{
				{
					Name: "new_file1.txt",
				},
				{
					Name: "new_file2.txt",
				},
				{
					Name: "new_file3.txt",
				},
			},
			[]*File{
				{
					Name: "new_file4.txt",
				},
				{
					Name: "new_file5.txt",
				},
				{
					Name: "new_file1.txt",
				},
			},
			func(files []*File) {
				expected := []*File{
					{
						Name: "new_file1.txt",
					},
					{
						Name: "new_file4.txt",
					},
					{
						Name: "new_file5.txt",
					},
				}

				assert.Len(t, files, 3)
				assert.EqualValues(t, expected, files)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()

			s.test(gitCmd.MergeStatusFiles(s.oldFiles, s.newFiles))
		})
	}
}

// TestGitCommandGetCommitDifferences is a function.
func TestGitCommandGetCommitDifferences(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(string, string)
	}

	scenarios := []scenario{
		{
			"Can't retrieve pushable count",
			func(string, ...string) *exec.Cmd {
				return exec.Command("test")
			},
			func(pushableCount string, pullableCount string) {
				assert.EqualValues(t, "?", pushableCount)
				assert.EqualValues(t, "?", pullableCount)
			},
		},
		{
			"Can't retrieve pullable count",
			func(cmd string, args ...string) *exec.Cmd {
				if args[1] == "HEAD..@{u}" {
					return exec.Command("test")
				}

				return exec.Command("echo")
			},
			func(pushableCount string, pullableCount string) {
				assert.EqualValues(t, "?", pushableCount)
				assert.EqualValues(t, "?", pullableCount)
			},
		},
		{
			"Retrieve pullable and pushable count",
			func(cmd string, args ...string) *exec.Cmd {
				if args[1] == "HEAD..@{u}" {
					return exec.Command("echo", "10")
				}

				return exec.Command("echo", "11")
			},
			func(pushableCount string, pullableCount string) {
				assert.EqualValues(t, "11", pushableCount)
				assert.EqualValues(t, "10", pullableCount)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.GetCommitDifferences("HEAD", "@{u}"))
		})
	}
}

// TestGitCommandGetCommitsToPush is a function.
func TestGitCommandGetCommitsToPush(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(map[string]bool)
	}

	scenarios := []scenario{
		{
			"Can't retrieve pushable commits",
			func(string, ...string) *exec.Cmd {
				return exec.Command("test")
			},
			func(pushables map[string]bool) {
				assert.EqualValues(t, map[string]bool{}, pushables)
			},
		},
		{
			"Retrieve pushable commits",
			func(cmd string, args ...string) *exec.Cmd {
				return exec.Command("echo", "8a2bb0e\n78976bc")
			},
			func(pushables map[string]bool) {
				assert.Len(t, pushables, 2)
				assert.EqualValues(t, map[string]bool{"8a2bb0e": true, "78976bc": true}, pushables)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.GetCommitsToPush())
		})
	}
}

// TestGitCommandRenameCommit is a function.
func TestGitCommandRenameCommit(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"commit", "--allow-empty", "--amend", "-m", "test"}, args)

		return exec.Command("echo")
	}

	assert.NoError(t, gitCmd.RenameCommit("test"))
}

// TestGitCommandResetToCommit is a function.
func TestGitCommandResetToCommit(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"reset", "78976bc"}, args)

		return exec.Command("echo")
	}

	assert.NoError(t, gitCmd.ResetToCommit("78976bc"))
}

// TestGitCommandNewBranch is a function.
func TestGitCommandNewBranch(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"checkout", "-b", "test"}, args)

		return exec.Command("echo")
	}

	assert.NoError(t, gitCmd.NewBranch("test"))
}

// TestGitCommandDeleteBranch is a function.
func TestGitCommandDeleteBranch(t *testing.T) {
	type scenario struct {
		testName string
		branch   string
		force    bool
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"Delete a branch",
			"test",
			false,
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"branch", "-d", "test"}, args)

				return exec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"Force delete a branch",
			"test",
			true,
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"branch", "-D", "test"}, args)

				return exec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.DeleteBranch(s.branch, s.force))
		})
	}
}

// TestGitCommandMerge is a function.
func TestGitCommandMerge(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"merge", "--no-edit", "test"}, args)

		return exec.Command("echo")
	}

	assert.NoError(t, gitCmd.Merge("test"))
}

// TestGitCommandUsingGpg is a function.
func TestGitCommandUsingGpg(t *testing.T) {
	type scenario struct {
		testName           string
		getLocalGitConfig  func(string) (string, error)
		getGlobalGitConfig func(string) (string, error)
		test               func(bool)
	}

	scenarios := []scenario{
		{
			"Option global and local config commit.gpgsign is not set",
			func(string) (string, error) {
				return "", nil
			},
			func(string) (string, error) {
				return "", nil
			},
			func(gpgEnabled bool) {
				assert.False(t, gpgEnabled)
			},
		},
		{
			"Option global config commit.gpgsign is not set, fallback on local config",
			func(string) (string, error) {
				return "", nil
			},
			func(string) (string, error) {
				return "true", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
		{
			"Option commit.gpgsign is true",
			func(string) (string, error) {
				return "True", nil
			},
			func(string) (string, error) {
				return "", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
		{
			"Option commit.gpgsign is on",
			func(string) (string, error) {
				return "ON", nil
			},
			func(string) (string, error) {
				return "", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
		{
			"Option commit.gpgsign is yes",
			func(string) (string, error) {
				return "YeS", nil
			},
			func(string) (string, error) {
				return "", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
		{
			"Option commit.gpgsign is 1",
			func(string) (string, error) {
				return "1", nil
			},
			func(string) (string, error) {
				return "", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.getGlobalGitConfig = s.getGlobalGitConfig
			gitCmd.getLocalGitConfig = s.getLocalGitConfig
			s.test(gitCmd.usingGpg())
		})
	}
}

// TestGitCommandCommit is a function.
func TestGitCommandCommit(t *testing.T) {
	type scenario struct {
		testName           string
		command            func(string, ...string) *exec.Cmd
		getGlobalGitConfig func(string) (string, error)
		test               func(*exec.Cmd, error)
	}

	scenarios := []scenario{
		{
			"Commit using gpg",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "bash", cmd)
				assert.EqualValues(t, []string{"-c", `git commit -m 'test'`}, args)

				return exec.Command("echo")
			},
			func(string) (string, error) {
				return "true", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NotNil(t, cmd)
				assert.Nil(t, err)
			},
		},
		{
			"Commit without using gpg",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "-m", "test"}, args)

				return exec.Command("echo")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Nil(t, err)
			},
		},
		{
			"Commit without using gpg with an error",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "-m", "test"}, args)

				return exec.Command("test")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Error(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.getGlobalGitConfig = s.getGlobalGitConfig
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.Commit("test", false))
		})
	}
}

// TestGitCommandCommitAmendFromFiles is a function.
func TestGitCommandCommitAmendFromFiles(t *testing.T) {
	type scenario struct {
		testName           string
		command            func(string, ...string) *exec.Cmd
		getGlobalGitConfig func(string) (string, error)
		test               func(*exec.Cmd, error)
	}

	scenarios := []scenario{
		{
			"Amend commit using gpg",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "bash", cmd)
				assert.EqualValues(t, []string{"-c", `git commit --amend -m 'test'`}, args)

				return exec.Command("echo")
			},
			func(string) (string, error) {
				return "true", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NotNil(t, cmd)
				assert.Nil(t, err)
			},
		},
		{
			"Amend commit without using gpg",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "--amend", "-m", "test"}, args)

				return exec.Command("echo")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Nil(t, err)
			},
		},
		{
			"Amend commit without using gpg with an error",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "--amend", "-m", "test"}, args)

				return exec.Command("test")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Error(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.getGlobalGitConfig = s.getGlobalGitConfig
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.Commit("test", true))
		})
	}
}

// TestGitCommandPush is a function.
func TestGitCommandPush(t *testing.T) {
	type scenario struct {
		testName  string
		command   func(string, ...string) *exec.Cmd
		forcePush bool
		test      func(error)
	}

	scenarios := []scenario{
		{
			"Push with force disabled",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"push", "-u", "origin", "test"}, args)

				return exec.Command("echo")
			},
			false,
			func(err error) {
				assert.Nil(t, err)
			},
		},
		{
			"Push with force enabled",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"push", "--force-with-lease", "-u", "origin", "test"}, args)

				return exec.Command("echo")
			},
			true,
			func(err error) {
				assert.Nil(t, err)
			},
		},
		{
			"Push with an error occurring",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"push", "-u", "origin", "test"}, args)

				return exec.Command("test")
			},
			false,
			func(err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.Push("test", s.forcePush))
		})
	}
}

// TestGitCommandSquashPreviousTwoCommits is a function.
func TestGitCommandSquashPreviousTwoCommits(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"Git reset triggers an error",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"reset", "--soft", "HEAD^"}, args)

				return exec.Command("test")
			},
			func(err error) {
				assert.NotNil(t, err)
			},
		},
		{
			"Git commit triggers an error",
			func(cmd string, args ...string) *exec.Cmd {
				if len(args) > 0 && args[0] == "reset" {
					return exec.Command("echo")
				}

				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "--amend", "-m", "test"}, args)

				return exec.Command("test")
			},
			func(err error) {
				assert.NotNil(t, err)
			},
		},
		{
			"Stash succeeded",
			func(cmd string, args ...string) *exec.Cmd {
				if len(args) > 0 && args[0] == "reset" {
					return exec.Command("echo")
				}

				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "--amend", "-m", "test"}, args)

				return exec.Command("echo")
			},
			func(err error) {
				assert.Nil(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.SquashPreviousTwoCommits("test"))
		})
	}
}

// TestGitCommandSquashFixupCommit is a function.
func TestGitCommandSquashFixupCommit(t *testing.T) {
	type scenario struct {
		testName string
		command  func() (func(string, ...string) *exec.Cmd, *[][]string)
		test     func(*[][]string, error)
	}

	scenarios := []scenario{
		{
			"An error occurred with one of the sub git command",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)
					if len(args) > 0 && args[0] == "checkout" {
						return exec.Command("test")
					}

					return exec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NotNil(t, err)
				assert.Len(t, *cmdsCalled, 3)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"checkout", "-q", "6789abcd"},
					{"branch", "-d", "6789abcd"},
					{"checkout", "test"},
				})
			},
		},
		{
			"Squash fixup succeeded",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)
					return exec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.Nil(t, err)
				assert.Len(t, *cmdsCalled, 4)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"checkout", "-q", "6789abcd"},
					{"reset", "--soft", "6789abcd^"},
					{"commit", "--amend", "-C", "6789abcd^"},
					{"rebase", "--onto", "HEAD", "6789abcd", "test"},
				})
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			var cmdsCalled *[][]string
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command, cmdsCalled = s.command()
			s.test(cmdsCalled, gitCmd.SquashFixupCommit("test", "6789abcd"))
		})
	}
}

// TestGitCommandCatFile is a function.
func TestGitCommandCatFile(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "cat", cmd)
		assert.EqualValues(t, []string{"test.txt"}, args)

		return exec.Command("echo", "-n", "test")
	}

	o, err := gitCmd.CatFile("test.txt")
	assert.NoError(t, err)
	assert.Equal(t, "test", o)
}

// TestGitCommandStageFile is a function.
func TestGitCommandStageFile(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"add", "test.txt"}, args)

		return exec.Command("echo")
	}

	assert.NoError(t, gitCmd.StageFile("test.txt"))
}

// TestGitCommandUnstageFile is a function.
func TestGitCommandUnstageFile(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
		tracked  bool
	}

	scenarios := []scenario{
		{
			"Remove an untracked file from staging",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"rm", "--cached", "test.txt"}, args)

				return exec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
			false,
		},
		{
			"Remove a tracked file from staging",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"reset", "HEAD", "test.txt"}, args)

				return exec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.UnStageFile("test.txt", s.tracked))
		})
	}
}

// TestGitCommandIsInMergeState is a function.
func TestGitCommandIsInMergeState(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(bool, error)
	}

	scenarios := []scenario{
		{
			"An error occurred when running status command",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"status", "--untracked-files=all"}, args)

				return exec.Command("test")
			},
			func(isInMergeState bool, err error) {
				assert.Error(t, err)
				assert.False(t, isInMergeState)
			},
		},
		{
			"Is not in merge state",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"status", "--untracked-files=all"}, args)
				return exec.Command("echo")
			},
			func(isInMergeState bool, err error) {
				assert.False(t, isInMergeState)
				assert.NoError(t, err)
			},
		},
		{
			"Command output contains conclude merge",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"status", "--untracked-files=all"}, args)
				return exec.Command("echo", "'conclude merge'")
			},
			func(isInMergeState bool, err error) {
				assert.True(t, isInMergeState)
				assert.NoError(t, err)
			},
		},
		{
			"Command output contains unmerged paths",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"status", "--untracked-files=all"}, args)
				return exec.Command("echo", "'unmerged paths'")
			},
			func(isInMergeState bool, err error) {
				assert.True(t, isInMergeState)
				assert.NoError(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.IsInMergeState())
		})
	}
}

// TestGitCommandRemoveFile is a function.
func TestGitCommandRemoveFile(t *testing.T) {
	type scenario struct {
		testName   string
		command    func() (func(string, ...string) *exec.Cmd, *[][]string)
		test       func(*[][]string, error)
		file       *File
		removeFile func(string) error
	}

	scenarios := []scenario{
		{
			"An error occurred when resetting",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return exec.Command("test")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.Error(t, err)
				assert.Len(t, *cmdsCalled, 1)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"reset", "--", "test"},
				})
			},
			&File{
				Name:             "test",
				HasStagedChanges: true,
			},
			func(string) error {
				return nil
			},
		},
		{
			"An error occurred when removing file",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return exec.Command("test")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "an error occurred when removing file")
				assert.Len(t, *cmdsCalled, 0)
			},
			&File{
				Name:    "test",
				Tracked: false,
			},
			func(string) error {
				return fmt.Errorf("an error occurred when removing file")
			},
		},
		{
			"An error occurred with checkout",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return exec.Command("test")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.Error(t, err)
				assert.Len(t, *cmdsCalled, 1)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"checkout", "--", "test"},
				})
			},
			&File{
				Name:             "test",
				Tracked:          true,
				HasStagedChanges: false,
			},
			func(string) error {
				return nil
			},
		},
		{
			"Checkout only",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return exec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 1)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"checkout", "--", "test"},
				})
			},
			&File{
				Name:             "test",
				Tracked:          true,
				HasStagedChanges: false,
			},
			func(string) error {
				return nil
			},
		},
		{
			"Reset and checkout",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return exec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 2)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"reset", "--", "test"},
					{"checkout", "--", "test"},
				})
			},
			&File{
				Name:             "test",
				Tracked:          true,
				HasStagedChanges: true,
			},
			func(string) error {
				return nil
			},
		},
		{
			"Reset and remove",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return exec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 1)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"reset", "--", "test"},
				})
			},
			&File{
				Name:             "test",
				Tracked:          false,
				HasStagedChanges: true,
			},
			func(filename string) error {
				assert.Equal(t, "test", filename)
				return nil
			},
		},
		{
			"Remove only",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return exec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 0)
			},
			&File{
				Name:             "test",
				Tracked:          false,
				HasStagedChanges: false,
			},
			func(filename string) error {
				assert.Equal(t, "test", filename)
				return nil
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			var cmdsCalled *[][]string
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command, cmdsCalled = s.command()
			gitCmd.removeFile = s.removeFile
			s.test(cmdsCalled, gitCmd.RemoveFile(s.file))
		})
	}
}

// TestGitCommandShow is a function.
func TestGitCommandShow(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"show", "--color", "456abcde"}, args)

		return exec.Command("echo")
	}

	_, err := gitCmd.Show("456abcde")
	assert.NoError(t, err)
}

// TestGitCommandCheckout is a function.
func TestGitCommandCheckout(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
		force    bool
	}

	scenarios := []scenario{
		{
			"Checkout",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"checkout", "test"}, args)

				return exec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
			false,
		},
		{
			"Checkout forced",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"checkout", "--force", "test"}, args)

				return exec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.Checkout("test", s.force))
		})
	}
}

// TestGitCommandGetBranchGraph is a function.
func TestGitCommandGetBranchGraph(t *testing.T) {
	gitCmd := newDummyGitCommand()
	gitCmd.OSCommand.command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"log", "--graph", "--color", "--abbrev-commit", "--decorate", "--date=relative", "--pretty=medium", "-100", "test"}, args)

		return exec.Command("echo")
	}

	_, err := gitCmd.GetBranchGraph("test")
	assert.NoError(t, err)
}

// TestGitCommandGetCommits is a function.
func TestGitCommandGetCommits(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func([]*Commit, error)
	}

	scenarios := []scenario{
		{
			"No data found",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "rev-list":
					assert.EqualValues(t, []string{"rev-list", "@{u}..HEAD", "--abbrev-commit"}, args)
					return exec.Command("echo")
				case "log":
					assert.EqualValues(t, []string{"log", "--oneline", "-30"}, args)
					return exec.Command("echo")
				case "merge-base":
					assert.EqualValues(t, []string{"merge-base", "HEAD", "master"}, args)
					return exec.Command("test")
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					return exec.Command("echo", "master")
				}

				return nil
			},
			func(commits []*Commit, err error) {
				assert.NoError(t, err)
				assert.Len(t, commits, 0)
			},
		},
		{
			"GetCommits returns 2 commits, 1 pushed the other not",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "rev-list":
					assert.EqualValues(t, []string{"rev-list", "@{u}..HEAD", "--abbrev-commit"}, args)
					return exec.Command("echo", "8a2bb0e")
				case "log":
					assert.EqualValues(t, []string{"log", "--oneline", "-30"}, args)
					return exec.Command("echo", "8a2bb0e commit 1\n78976bc commit 2")
				case "merge-base":
					assert.EqualValues(t, []string{"merge-base", "HEAD", "master"}, args)
					return exec.Command("echo", "78976bc")
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					return exec.Command("echo", "master")
				}

				return nil
			},
			func(commits []*Commit, err error) {
				assert.NoError(t, err)
				assert.Len(t, commits, 2)
				assert.EqualValues(t, []*Commit{
					{
						Sha:           "8a2bb0e",
						Name:          "commit 1",
						Pushed:        true,
						Merged:        false,
						DisplayString: "8a2bb0e commit 1",
					},
					{
						Sha:           "78976bc",
						Name:          "commit 2",
						Pushed:        false,
						Merged:        true,
						DisplayString: "78976bc commit 2",
					},
				}, commits)
			},
		},
		{
			"GetCommits bubbles up an error from setCommitMergedStatuses",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "rev-list":
					assert.EqualValues(t, []string{"rev-list", "@{u}..HEAD", "--abbrev-commit"}, args)
					return exec.Command("echo", "8a2bb0e")
				case "log":
					assert.EqualValues(t, []string{"log", "--oneline", "-30"}, args)
					return exec.Command("echo", "8a2bb0e commit 1\n78976bc commit 2")
				case "merge-base":
					assert.EqualValues(t, []string{"merge-base", "HEAD", "master"}, args)
					return exec.Command("echo", "78976bc")
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					// here's where we are returning the error
					return exec.Command("test")
				case "rev-parse":
					assert.EqualValues(t, []string{"rev-parse", "--short", "HEAD"}, args)
					// here too
					return exec.Command("test")
				}

				return nil
			},
			func(commits []*Commit, err error) {
				assert.Error(t, err)
				assert.Len(t, commits, 0)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.GetCommits())
		})
	}
}

// TestGitCommandGetLog is a function.
func TestGitCommandGetLog(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(string)
	}

	scenarios := []scenario{
		{
			"Retrieves logs",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"log", "--oneline", "-30"}, args)

				return exec.Command("echo", "6f0b32f commands/git : add GetCommits tests refactor\n9d9d775 circle : remove new line")
			},
			func(output string) {
				assert.EqualValues(t, "6f0b32f commands/git : add GetCommits tests refactor\n9d9d775 circle : remove new line\n", output)
			},
		},
		{
			"An error occurred when retrieving logs",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"log", "--oneline", "-30"}, args)
				return exec.Command("test")
			},
			func(output string) {
				assert.Empty(t, output)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.GetLog())
		})
	}
}

// TestGitCommandDiff is a function.
func TestGitCommandDiff(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		file     *File
		plain    bool
	}

	scenarios := []scenario{
		{
			"Default case",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"diff", "--color", "--", "test.txt"}, args)

				return exec.Command("echo")
			},
			&File{
				Name:             "test.txt",
				HasStagedChanges: false,
				Tracked:          true,
			},
			false,
		},
		{
			"Default case",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"diff", "--", "test.txt"}, args)

				return exec.Command("echo")
			},
			&File{
				Name:             "test.txt",
				HasStagedChanges: false,
				Tracked:          true,
			},
			true,
		},
		{
			"All changes staged",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"diff", "--color", "--cached", "--", "test.txt"}, args)

				return exec.Command("echo")
			},
			&File{
				Name:               "test.txt",
				HasStagedChanges:   true,
				HasUnstagedChanges: false,
				Tracked:            true,
			},
			false,
		},
		{
			"File not tracked and file has no staged changes",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"diff", "--color", "--no-index", "/dev/null", "test.txt"}, args)

				return exec.Command("echo")
			},
			&File{
				Name:             "test.txt",
				HasStagedChanges: false,
				Tracked:          false,
			},
			false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			gitCmd.Diff(s.file, s.plain)
		})
	}
}

// TestGitCommandGetMergeBase is a function.
func TestGitCommandGetMergeBase(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(string, error)
	}

	scenarios := []scenario{
		{
			"swallows an error if the call to merge-base returns an error",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					return exec.Command("echo", "master")
				case "merge-base":
					assert.EqualValues(t, []string{"merge-base", "HEAD", "master"}, args)
					return exec.Command("test")
				}
				return nil
			},
			func(output string, err error) {
				assert.NoError(t, err)
				assert.EqualValues(t, "", output)
			},
		},
		{
			"returns the commit when master",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					return exec.Command("echo", "master")
				case "merge-base":
					assert.EqualValues(t, []string{"merge-base", "HEAD", "master"}, args)
					return exec.Command("echo", "blah")
				}
				return nil
			},
			func(output string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "blah\n", output)
			},
		},
		{
			"checks against develop when a feature branch",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					return exec.Command("echo", "feature/test")
				case "merge-base":
					assert.EqualValues(t, []string{"merge-base", "HEAD", "develop"}, args)
					return exec.Command("echo", "blah")
				}
				return nil
			},
			func(output string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "blah\n", output)
			},
		},
		{
			"bubbles up error if there is one",
			func(cmd string, args ...string) *exec.Cmd {
				return exec.Command("test")
			},
			func(output string, err error) {
				assert.Error(t, err)
				assert.Equal(t, "", output)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.getMergeBase())
		})
	}
}

// TestGitCommandCurrentBranchName is a function.
func TestGitCommandCurrentBranchName(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(string, error)
	}

	scenarios := []scenario{
		{
			"says we are on the master branch if we are",
			func(cmd string, args ...string) *exec.Cmd {
				assert.Equal(t, "git", cmd)
				return exec.Command("echo", "master")
			},
			func(output string, err error) {
				assert.NoError(t, err)
				assert.EqualValues(t, "master", output)
			},
		},
		{
			"falls back to git rev-parse if symbolic-ref fails",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					return exec.Command("test")
				case "rev-parse":
					assert.EqualValues(t, []string{"rev-parse", "--short", "HEAD"}, args)
					return exec.Command("echo", "master")
				}

				return nil
			},
			func(output string, err error) {
				assert.NoError(t, err)
				assert.EqualValues(t, "master", output)
			},
		},
		{
			"bubbles up error if there is one",
			func(cmd string, args ...string) *exec.Cmd {
				assert.Equal(t, "git", cmd)
				return exec.Command("test")
			},
			func(output string, err error) {
				assert.Error(t, err)
				assert.EqualValues(t, "", output)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.CurrentBranchName())
		})
	}
}

func TestGitCommandApplyPatch(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(string, error)
	}

	scenarios := []scenario{
		{
			"valid case",
			func(cmd string, args ...string) *exec.Cmd {
				assert.Equal(t, "git", cmd)
				assert.EqualValues(t, []string{"apply", "--cached"}, args[0:2])
				filename := args[2]
				content, err := ioutil.ReadFile(filename)
				assert.NoError(t, err)

				assert.Equal(t, "test", string(content))

				return exec.Command("echo", "done")
			},
			func(output string, err error) {
				assert.NoError(t, err)
				assert.EqualValues(t, "done\n", output)
			},
		},
		{
			"command returns error",
			func(cmd string, args ...string) *exec.Cmd {
				assert.Equal(t, "git", cmd)
				assert.EqualValues(t, []string{"apply", "--cached"}, args[0:2])
				filename := args[2]
				// TODO: Ideally we want to mock out OSCommand here so that we're not
				// double handling testing it's CreateTempFile functionality,
				// but it is going to take a bit of work to make a proper mock for it
				// so I'm leaving it for another PR
				content, err := ioutil.ReadFile(filename)
				assert.NoError(t, err)

				assert.Equal(t, "test", string(content))

				return exec.Command("test")
			},
			func(output string, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := newDummyGitCommand()
			gitCmd.OSCommand.command = s.command
			s.test(gitCmd.ApplyPatch("test"))
		})
	}
}
