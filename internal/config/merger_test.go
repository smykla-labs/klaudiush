package config_test

import (
	"testing"

	"github.com/smykla-labs/klaudiush/internal/config"
	pkgconfig "github.com/smykla-labs/klaudiush/pkg/config"
)

func TestMerger_MergeMarkdownConfig(t *testing.T) {
	t.Run("merges MarkdownlintRules map", func(t *testing.T) {
		merger := config.NewMerger()

		globalCfg := &pkgconfig.Config{
			Validators: &pkgconfig.ValidatorsConfig{
				File: &pkgconfig.FileConfig{
					Markdown: &pkgconfig.MarkdownValidatorConfig{
						MarkdownlintRules: map[string]bool{
							"MD013": false,
							"MD034": false,
						},
					},
				},
			},
		}

		projectCfg := &pkgconfig.Config{}

		result := merger.Merge(globalCfg, projectCfg)

		if result.Validators.File.Markdown == nil {
			t.Fatal("Markdown config is nil")
		}

		rules := result.Validators.File.Markdown.MarkdownlintRules
		if len(rules) != 2 {
			t.Errorf("Expected 2 rules, got %d", len(rules))
		}

		if rules["MD013"] != false {
			t.Error("MD013 should be false")
		}

		if rules["MD034"] != false {
			t.Error("MD034 should be false")
		}
	})

	t.Run("project config overrides global MarkdownlintRules", func(t *testing.T) {
		merger := config.NewMerger()

		globalCfg := &pkgconfig.Config{
			Validators: &pkgconfig.ValidatorsConfig{
				File: &pkgconfig.FileConfig{
					Markdown: &pkgconfig.MarkdownValidatorConfig{
						MarkdownlintRules: map[string]bool{
							"MD013": false,
						},
					},
				},
			},
		}

		projectCfg := &pkgconfig.Config{
			Validators: &pkgconfig.ValidatorsConfig{
				File: &pkgconfig.FileConfig{
					Markdown: &pkgconfig.MarkdownValidatorConfig{
						MarkdownlintRules: map[string]bool{
							"MD013": true,
							"MD041": false,
						},
					},
				},
			},
		}

		result := merger.Merge(globalCfg, projectCfg)

		rules := result.Validators.File.Markdown.MarkdownlintRules
		if len(rules) != 2 {
			t.Errorf("Expected 2 rules, got %d", len(rules))
		}

		if rules["MD013"] != true {
			t.Error("MD013 should be true (overridden by project)")
		}

		if rules["MD041"] != false {
			t.Error("MD041 should be false")
		}
	})

	t.Run("merges MarkdownlintPath", func(t *testing.T) {
		merger := config.NewMerger()

		cfg := &pkgconfig.Config{
			Validators: &pkgconfig.ValidatorsConfig{
				File: &pkgconfig.FileConfig{
					Markdown: &pkgconfig.MarkdownValidatorConfig{
						MarkdownlintPath: "/custom/path/to/markdownlint",
					},
				},
			},
		}

		result := merger.Merge(cfg)

		if result.Validators.File.Markdown.MarkdownlintPath != "/custom/path/to/markdownlint" {
			t.Errorf("MarkdownlintPath not merged correctly")
		}
	})

	t.Run("merges MarkdownlintConfig", func(t *testing.T) {
		merger := config.NewMerger()

		cfg := &pkgconfig.Config{
			Validators: &pkgconfig.ValidatorsConfig{
				File: &pkgconfig.FileConfig{
					Markdown: &pkgconfig.MarkdownValidatorConfig{
						MarkdownlintConfig: ".markdownlint.json",
					},
				},
			},
		}

		result := merger.Merge(cfg)

		if result.Validators.File.Markdown.MarkdownlintConfig != ".markdownlint.json" {
			t.Errorf("MarkdownlintConfig not merged correctly")
		}
	})
}

func TestMerger_MergePRConfig(t *testing.T) {
	t.Run("merges MarkdownDisabledRules", func(t *testing.T) {
		merger := config.NewMerger()

		cfg := &pkgconfig.Config{
			Validators: &pkgconfig.ValidatorsConfig{
				Git: &pkgconfig.GitConfig{
					PR: &pkgconfig.PRValidatorConfig{
						MarkdownDisabledRules: []string{"MD013", "MD034", "MD041"},
					},
				},
			},
		}

		result := merger.Merge(cfg)

		rules := result.Validators.Git.PR.MarkdownDisabledRules
		if len(rules) != 3 {
			t.Errorf("Expected 3 rules, got %d", len(rules))
		}
	})
}
