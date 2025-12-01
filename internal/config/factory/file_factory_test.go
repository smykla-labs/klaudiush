package factory_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-labs/klaudiush/internal/config/factory"
	"github.com/smykla-labs/klaudiush/pkg/config"
	"github.com/smykla-labs/klaudiush/pkg/logger"
)

var _ = Describe("FileValidatorFactory", func() {
	var (
		fileFactory *factory.FileValidatorFactory
		log         logger.Logger
	)

	BeforeEach(func() {
		log = logger.NewNoOpLogger()
		fileFactory = factory.NewFileValidatorFactory(log)
	})

	Describe("NewFileValidatorFactory", func() {
		It("should create a new file validator factory", func() {
			Expect(fileFactory).NotTo(BeNil())
		})
	})

	Describe("SetRuleEngine", func() {
		It("should set rule engine on factory", func() {
			enabled := true
			rulesCfg := &config.Config{
				Rules: &config.RulesConfig{
					Enabled: &enabled,
					Rules: []config.RuleConfig{
						{
							Name:   "test-rule",
							Action: &config.RuleActionConfig{Type: "block"},
						},
					},
				},
			}

			rulesFactory := factory.NewRulesFactory(log)
			engine, err := rulesFactory.CreateRuleEngine(rulesCfg)
			Expect(err).NotTo(HaveOccurred())

			// Should not panic
			fileFactory.SetRuleEngine(engine)
		})

		It("should handle nil rule engine", func() {
			// Should not panic
			fileFactory.SetRuleEngine(nil)
		})
	})

	Describe("CreateValidators", func() {
		Context("when gofumpt validator is enabled", func() {
			It("should create gofumpt validator", func() {
				cfg := &config.Config{
					Validators: &config.ValidatorsConfig{
						File: &config.FileConfig{
							Gofumpt: &config.GofumptValidatorConfig{
								ValidatorConfig: config.ValidatorConfig{Enabled: ptrBool(true)},
							},
						},
					},
				}

				validators := fileFactory.CreateValidators(cfg)
				Expect(len(validators)).To(BeNumerically(">=", 1))
			})

			It("should configure gofumpt validator with options", func() {
				extraRules := true
				cfg := &config.Config{
					Validators: &config.ValidatorsConfig{
						File: &config.FileConfig{
							Gofumpt: &config.GofumptValidatorConfig{
								ValidatorConfig: config.ValidatorConfig{Enabled: ptrBool(true)},
								ExtraRules:      &extraRules,
								Lang:            "go1.21",
								ModPath:         "github.com/example/repo",
							},
						},
					},
				}

				validators := fileFactory.CreateValidators(cfg)
				Expect(len(validators)).To(BeNumerically(">=", 1))
			})
		})

		Context("when gofumpt validator is disabled", func() {
			It("should not create gofumpt validator", func() {
				cfg := &config.Config{
					Validators: &config.ValidatorsConfig{
						File: &config.FileConfig{
							Gofumpt: &config.GofumptValidatorConfig{
								ValidatorConfig: config.ValidatorConfig{Enabled: ptrBool(false)},
							},
						},
					},
				}

				validators := fileFactory.CreateValidators(cfg)
				Expect(len(validators)).To(Equal(0))
			})
		})

		Context("when gofumpt config is nil", func() {
			It("should not create gofumpt validator", func() {
				cfg := &config.Config{
					Validators: &config.ValidatorsConfig{
						File: &config.FileConfig{
							Gofumpt: nil,
						},
					},
				}

				validators := fileFactory.CreateValidators(cfg)
				Expect(len(validators)).To(Equal(0))
			})
		})

		Context("when rule engine is configured", func() {
			It("should attach rule adapter to gofumpt validator", func() {
				enabled := true
				rulesCfg := &config.Config{
					Rules: &config.RulesConfig{
						Enabled: &enabled,
						Rules: []config.RuleConfig{
							{
								Name:   "test-rule",
								Action: &config.RuleActionConfig{Type: "block"},
							},
						},
					},
					Validators: &config.ValidatorsConfig{
						File: &config.FileConfig{
							Gofumpt: &config.GofumptValidatorConfig{
								ValidatorConfig: config.ValidatorConfig{Enabled: ptrBool(true)},
							},
						},
					},
				}

				rulesFactory := factory.NewRulesFactory(log)
				engine, err := rulesFactory.CreateRuleEngine(rulesCfg)
				Expect(err).NotTo(HaveOccurred())

				fileFactory.SetRuleEngine(engine)
				validators := fileFactory.CreateValidators(rulesCfg)

				Expect(len(validators)).To(BeNumerically(">=", 1))
			})
		})
	})
})
