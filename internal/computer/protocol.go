package computer

// ProtocolVersion is negotiated independently from application releases.
const ProtocolVersion = 1

type Capabilities struct {
	ProtocolVersion int                `json:"protocol_version"`
	Preview         bool               `json:"preview"`
	Available       bool               `json:"available"`
	ReasonCode      string             `json:"reason_code"`
	LocalOnly       bool               `json:"local_only"`
	ApprovalMode    string             `json:"approval_mode"`
	Drivers         []DriverCapability `json:"drivers"`
}

type DriverCapability struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Qualified bool   `json:"qualified"`
}

// Planned support is not installed support. Until a paired, version-checked
// companion enforces scope and consent, every execution surface is disabled.
func UnavailableCapabilities() Capabilities {
	return Capabilities{
		ProtocolVersion: ProtocolVersion, Preview: true, ReasonCode: "companion_unavailable",
		LocalOnly: true, ApprovalMode: "supervised",
		Drivers: []DriverCapability{{ID: "browser"}, {ID: "windows-uia"}, {ID: "macos-accessibility"}, {ID: "linux-atspi"}},
	}
}
