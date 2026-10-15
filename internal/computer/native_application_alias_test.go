package computer

import (
	"fmt"
	"testing"
)

func TestApplicationAliasesAreBoundedRegisteredNames(t *testing.T) {
	app := NativeApplication{ID: "host-id", Name: "Editor"}
	for _, alias := range []string{"EDITOR", "", " ", `C:\Apps\editor.exe`, "/Applications/Editor", "bad\nname"} {
		addApplicationAlias(&app, alias)
	}
	if len(app.Aliases) != 0 {
		t.Fatal("invalid or redundant alias", app)
	}
	addApplicationAlias(&app, " Éditeur ")
	addApplicationAlias(&app, "ÉDITEUR")
	addApplicationAlias(&app, "エディター")
	if len(app.Aliases) != 2 || app.Aliases[0] != "Éditeur" {
		t.Fatal("localized alias lost or duplicated", app)
	}
	for i := 0; i < 20; i++ {
		addApplicationAlias(&app, fmt.Sprintf("Registered name %d", i))
	}
	if len(app.Aliases) != 8 || app.ID != "host-id" || app.Name != "Editor" {
		t.Fatal("aliases changed identity or exceeded bound", app)
	}
}
