package bypass

import "fmt"

// Notice returns the user-only reminder shown while klaudiush keeps validating
// a session that runs without approval prompts. It is emitted in the
// systemMessage field, which the AI never sees.
func Notice(mode string) string {
	return fmt.Sprintf(
		"klaudiush still validates with approval prompts off (%s).\n"+
			"Not what you want? klaudiush bypass skip [--global]\n"+
			"Keep validating, hide this note: klaudiush bypass notify off",
		mode,
	)
}
