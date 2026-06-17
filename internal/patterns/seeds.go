package patterns

import "time"

// seedCount is the initial count for seed patterns, above the default min_count of 3.
const seedCount = 5

// seedPairs defines source->target failure cascades.
// Each pair represents a commonly observed sequence where fixing the source error
// causes the target error to appear.
var seedPairs = [][2]string{
	// Git commit cascades
	{codeGIT013, codeGIT004}, // conventional format fix -> title too long
	{codeGIT004, codeGIT005}, // shorten title -> body line too long
	{codeGIT005, "GIT016"},   // body line fix -> list format issue
	{codeGIT013, "GIT006"},   // conventional format fix -> infra scope misuse
	{codeGIT010, codeGIT013}, // adding missing flags -> conventional format
	{codeGIT010, codeGIT004}, // adding flags pushes title over 50 chars
	// File linter cascades
	{"FILE006", codeFILE005}, // gofumpt reformats doc comments -> markdown lint
	{"FILE002", "FILE003"},   // terraform fmt passes -> tflint catches issues
	{codeFILE010, "FILE007"}, // removing noqa directive -> ruff error
	{codeFILE010, "FILE008"}, // removing eslint-disable -> oxlint error
	// Cross-category: secrets to shell
	{"SEC001", codeSHELL001}, // moving API key to env var -> command substitution
	{"SEC004", codeSHELL001}, // moving token to env var -> command substitution
	// Cross-category: PR then markdown
	{"GIT023", codeFILE005}, // fixing PR body formatting -> markdown lint
}

// SeedPatterns returns the built-in seed patterns.
// These represent commonly observed failure cascades.
func SeedPatterns() *PatternData {
	now := time.Now()
	patterns := make(map[string]*FailurePattern, len(seedPairs))

	for _, pair := range seedPairs {
		key := pair[0] + "->" + pair[1]
		patterns[key] = &FailurePattern{
			SourceCode: pair[0],
			TargetCode: pair[1],
			Count:      seedCount,
			FirstSeen:  now,
			LastSeen:   now,
			Seed:       true,
		}
	}

	return &PatternData{
		Patterns:    patterns,
		LastUpdated: now,
		Version:     patternDataVersion,
	}
}

// EnsureSeedData writes seed patterns to the project store if not already present.
func EnsureSeedData(store *FilePatternStore) error {
	if store.HasProjectData() {
		return nil
	}

	store.SetProjectData(SeedPatterns())

	return store.SaveProject()
}
