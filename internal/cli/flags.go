package cli

// GlobalFlags are the flags accepted before or after any subcommand
// (PRD §14, RF-12). Sprint 1 only parses them and threads them through —
// accounts/account show are read-only and don't yet branch on them.
type GlobalFlags struct {
	NonInteractive bool
	Yes            bool
	Verbose        bool
}

// extractGlobalFlags pulls the recognized global flags out of args,
// wherever they appear, and returns the remaining arguments in their
// original order for the subcommand dispatcher to interpret.
func extractGlobalFlags(args []string) (GlobalFlags, []string) {
	var flags GlobalFlags
	rest := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "--non-interactive":
			flags.NonInteractive = true
		case "--yes":
			flags.Yes = true
		case "--verbose":
			flags.Verbose = true
		default:
			rest = append(rest, a)
		}
	}
	return flags, rest
}
