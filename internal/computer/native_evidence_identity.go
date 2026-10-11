package computer

// CarryControlIdentities preserves opaque evidence identities only for unique
// OS identities seen in consecutive views. Keys are private provider identities,
// not names/values/positions. Callers retain OS references during the comparison
// and reset the previous map on target changes. Missing/duplicate keys fail closed.
func CarryControlIdentities(previous map[string]string, keys []string) []string {
	counts := map[string]int{}
	for _, key := range keys {
		counts[key]++
	}
	result := make([]string, len(keys))
	for i, key := range keys {
		if key == "" || counts[key] != 1 {
			continue
		}
		result[i] = previous[key]
		if result[i] == "" {
			result[i] = nativeID()
		}
	}
	return result
}
