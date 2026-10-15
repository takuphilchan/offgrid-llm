package computer

// Reviewed WinUI TabView template relationships, not localized UI labels.
// The AddButton semantics are app-specific: an arbitrary WinUI app can bind
// that button to anything. Only a validated packaged Notepad identity may use
// this profile. Package revision is sealed into every creation preparation.
// This is an implementation profile, not installed-application qualification.
func notepadDocumentProfile(aumid, fullName string) bool {
	return aumid == "Microsoft.WindowsNotepad_8wekyb3d8bbwe!App" &&
		fullName == "Microsoft.WindowsNotepad_11.2607.14.0_x64__8wekyb3d8bbwe"
}

func winUITabClass(name, suffix string) bool {
	return name == suffix || name == "Microsoft.UI.Xaml.Controls."+suffix || name == "Windows.UI.Xaml.Controls."+suffix
}

// Apply only after OS-backed application identity validation. All required
// controls must occur exactly once in the same bounded, complete raw tree.
func resolveWinUIDocumentProfile(nodes []documentNode, revision string) ([]documentNode, []DocumentCreationCommand, error) {
	copyNodes := append([]documentNode(nil), nodes...)
	byID := map[string]documentNode{}
	group, pane, command, active := "", "", "", ""
	for _, n := range nodes {
		byID[n.key] = n
		if winUITabClass(n.className, "TabView") {
			if group != "" {
				return nil, nil, ErrDocumentUnavailable
			}
			group = n.key
		}
	}
	if group == "" {
		return nil, nil, ErrDocumentUnavailable
	}
	within := func(key string) bool {
		for i := 0; key != "" && i < 64; i++ {
			if key == group {
				return true
			}
			n, ok := byID[key]
			if !ok {
				return false
			}
			key = n.parent
		}
		return false
	}
	for _, n := range nodes {
		if !within(n.key) {
			continue
		}
		if n.tab {
			if n.group != group || !winUITabClass(n.className, "TabViewItem") {
				return nil, nil, ErrDocumentUnavailable
			}
			if n.selected {
				if active != "" {
					return nil, nil, ErrDocumentChanged
				}
				active = n.key
			}
		}
		if n.automationID == "TabContentPresenter" && n.visible && !n.protected {
			if pane != "" {
				return nil, nil, ErrDocumentUnavailable
			}
			pane = n.key
		}
		if n.automationID == "AddButton" && n.visible && !n.protected && n.invokable {
			if command != "" {
				return nil, nil, ErrDocumentUnavailable
			}
			command = n.key
		}
	}
	if active == "" || pane == "" {
		return nil, nil, ErrDocumentUnavailable
	}
	for i := range copyNodes {
		if copyNodes[i].key == active {
			copyNodes[i].controls = append(append([]string{}, copyNodes[i].controls...), pane)
		}
	}
	if _, err := resolveDocumentTopology(copyNodes, true); err != nil {
		return nil, nil, err
	}
	commands := []DocumentCreationCommand{}
	if command != "" && revision != "" {
		commands = append(commands, DocumentCreationCommand{ID: nativeHash([]string{command, revision, "new_document"}), Element: command, Kind: "new_document", Profile: "winui-notepad-tabs-1", Revision: revision})
	}
	return copyNodes, commands, nil
}
