package git_test

import (
	"context"
	"os"
	"path/filepath"

	gogit "github.com/go-git/go-git/v6"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gitpkg "github.com/smykla-skalski/klaudiush/internal/git"
	"github.com/smykla-skalski/klaudiush/internal/validators/git"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

var _ = Describe("CommitValidator worktree staging check", func() {
	var (
		worktreeDir string
		repo        *gogit.Repository
		err         error
	)

	BeforeEach(func() {
		worktreeDir, err = os.MkdirTemp("", "commit-worktree-test-*")
		Expect(err).NotTo(HaveOccurred())

		worktreeDir, err = filepath.EvalSymlinks(worktreeDir)
		Expect(err).NotTo(HaveOccurred())

		repo, err = gogit.PlainInit(worktreeDir, false)
		Expect(err).NotTo(HaveOccurred())

		cfg, err := repo.Config()
		Expect(err).NotTo(HaveOccurred())

		cfg.User.Name = "Test User"
		cfg.User.Email = "test@example.com"

		err = repo.SetConfig(cfg)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if worktreeDir != "" {
			Expect(os.RemoveAll(worktreeDir)).To(Succeed())
		}
	})

	It("passes when worktree has staged files but hook cwd does not", func() {
		// Stage a file in the worktree
		testFile := filepath.Join(worktreeDir, "main.go")
		Expect(os.WriteFile(testFile, []byte("package main\n"), 0o644)).To(Succeed())

		wt, err := repo.Worktree()
		Expect(err).NotTo(HaveOccurred())

		_, err = wt.Add("main.go")
		Expect(err).NotTo(HaveOccurred())

		// Validator's own gitRunner has no staged files (simulates hook cwd = different repo)
		fakeGit := gitpkg.NewFakeRunner()
		fakeGit.StagedFiles = []string{}

		log := logger.NewNoOpLogger()
		v := git.NewCommitValidator(log, fakeGit, nil, nil)

		ctx := &hook.Context{
			EventType: hook.EventTypePreToolUse,
			ToolName:  hook.ToolTypeBash,
			ToolInput: hook.ToolInput{
				Command: `cd ` + worktreeDir + ` && git commit -sS -m "feat(main): add entry point"`,
			},
		}

		// Without the fix: gitRunner (fakeGit) has no staged files → GIT003
		// With the fix:    runner scoped to worktreeDir finds staged main.go → pass
		result := v.Validate(context.Background(), ctx)
		Expect(result.Passed).To(BeTrue())
	})

	It("blocks when worktree has no staged files", func() {
		fakeGit := gitpkg.NewFakeRunner()
		fakeGit.StagedFiles = []string{"irrelevant.go"} // hook cwd has staged files but wrong dir

		log := logger.NewNoOpLogger()
		v := git.NewCommitValidator(log, fakeGit, nil, nil)

		ctx := &hook.Context{
			EventType: hook.EventTypePreToolUse,
			ToolName:  hook.ToolTypeBash,
			ToolInput: hook.ToolInput{
				Command: `cd ` + worktreeDir + ` && git commit -sS -m "feat(main): add entry point"`,
			},
		}

		result := v.Validate(context.Background(), ctx)
		Expect(result.Passed).To(BeFalse())
		Expect(result.Message).To(ContainSubstring("No files staged"))
	})

	It("uses gitRunnerFor with -C flag (not cd) when worktree has staged files", func() {
		testFile := filepath.Join(worktreeDir, "main.go")
		Expect(os.WriteFile(testFile, []byte("package main\n"), 0o644)).To(Succeed())

		wt, err := repo.Worktree()
		Expect(err).NotTo(HaveOccurred())

		_, err = wt.Add("main.go")
		Expect(err).NotTo(HaveOccurred())

		fakeGit := gitpkg.NewFakeRunner()
		fakeGit.StagedFiles = []string{}

		log := logger.NewNoOpLogger()
		v := git.NewCommitValidator(log, fakeGit, nil, nil)

		ctx := &hook.Context{
			EventType: hook.EventTypePreToolUse,
			ToolName:  hook.ToolTypeBash,
			ToolInput: hook.ToolInput{
				Command: `git -C ` + worktreeDir + ` commit -sS -m "feat(main): add entry point"`,
			},
		}

		result := v.Validate(context.Background(), ctx)
		Expect(result.Passed).To(BeTrue())
	})
})
