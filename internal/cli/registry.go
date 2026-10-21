package cli

// commandNames is the authoritative root command/alias inventory. It does not
// execute commands; both the transitional cmd adapter and Execute resolve
// through this table so aliases cannot silently diverge during migration.
var commandNames = []struct {
	Name    string
	Aliases []string
}{
	{"recommend", []string{"auto-select", "autoselect"}},
	{"doctor", []string{"check", "diagnose"}},
	{"appliance", []string{"box"}},
	{"init", []string{"setup"}},
	{"download", nil},
	{"download-hf", nil},
	{"search", nil},
	{"run", nil},
	{"import", nil},
	{"remove", []string{"delete", "rm"}},
	{"export", nil},
	{"chat", nil},
	{"benchmark", []string{"bench"}},
	{"compare", []string{"benchmark-compare"}},
	{"test", nil},
	{"list", nil},
	{"model", []string{"models"}},
	{"verify", nil},
	{"shell-completion", nil},
	{"export-session", nil},
	{"quantization", []string{"quant"}},
	{"quantize", nil},
	{"info", []string{"status"}},
	{"config", nil},
	{"peers", []string{"p2p"}},
	{"alias", nil},
	{"favorite", []string{"fav"}},
	{"template", []string{"tpl"}},
	{"batch", nil},
	{"session", []string{"sessions"}},
	{"kb", []string{"knowledge", "rag"}},
	{"workspace", nil},
	{"completions", []string{"completion"}},
	{"users", []string{"user"}},
	{"audit", nil},
	{"metrics", nil},
	{"lora", nil},
	{"agent", []string{"agents"}},
	{"integration", []string{"integrations"}},
	{"computer", nil},
	{"hermes", nil},
	{"openclaw", nil},
	{"audio", nil},
	{"logs", []string{"log"}},
	{"serve", []string{"server"}},
	{"version", []string{"-v", "--version"}},
	{"help", []string{"-h", "--help"}},
}

var commandAliases = func() map[string]string {
	result := make(map[string]string)
	for _, command := range commandNames {
		for _, name := range append([]string{command.Name}, command.Aliases...) {
			if _, exists := result[name]; exists {
				panic("duplicate CLI command alias: " + name)
			}
			result[name] = command.Name
		}
	}
	return result
}()

// CanonicalCommand returns the single dispatch name for a known alias.
func CanonicalCommand(name string) (string, bool) {
	canonical, ok := commandAliases[name]
	return canonical, ok
}
