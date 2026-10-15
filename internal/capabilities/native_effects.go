package capabilities

// NativeEffectsRevision identifies the reviewed operation semantics, not an
// application version or a permission grant. Unknown revisions fail closed.
const NativeEffectsRevision = "native-effects-v1"

type NativeEffect string

const (
	NativeRead   NativeEffect = "read"
	NativeChange NativeEffect = "change"
)

// NativeOperationEffect is shared by descriptor construction, durable intent
// classification and the native transport's action-budget accounting. Browser
// operations deliberately retain their existing recovery behavior.
func NativeOperationEffect(name, revision string) (NativeEffect, bool) {
	if revision != NativeEffectsRevision {
		return "", false
	}
	switch name {
	case "computer_observe", "computer_verify", "computer_verify_checked",
		"computer_applications", "computer_find_applications", "computer_windows", "computer_verify_application",
		"computer_find_controls", "computer_inspect_control", "computer_documents", "computer_read_text":
		return NativeRead, true
	case "computer_open_application", "computer_focus_window", "computer_replace_text",
		"computer_activate", "computer_set_checked", "computer_shortcut",
		"computer_create_document", "computer_select_document", "computer_select_text", "computer_insert_text", "computer_replace_selection":
		return NativeChange, true
	}
	return "", false
}

// TrustedNativeRead accepts only descriptors authored by our companion adapter.
// Neither a familiar name nor a remote readOnlyHint/low-risk claim is authority.
func TrustedNativeRead(d Descriptor) bool {
	if d.Source != "local-companion" || d.Namespace != "computer" || d.Kind != Computer || d.Risk != RiskLow {
		return false
	}
	effect, known := NativeOperationEffect(d.Name, d.Revision)
	return known && effect == NativeRead
}
