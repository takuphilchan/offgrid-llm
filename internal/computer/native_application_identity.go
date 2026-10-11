package computer

import (
	"os"
	"strings"
)

// These identities are host-owned and never accepted from a model. An empty
// kind denotes the original executable-only catalog, not a packaged fallback.
type applicationIdentityKind string

const (
	applicationExecutable applicationIdentityKind = "executable"
	applicationPackaged   applicationIdentityKind = "packaged"
)

type packagedApplication struct {
	AUMID            string
	Family           string
	FullName         string
	Entry            string
	Root             string
	ManifestRevision string
}

func (p packagedApplication) valid() bool {
	return p.Family != "" && p.Entry != "" && p.AUMID == p.Family+"!"+p.Entry &&
		p.FullName != "" && p.Root != "" && len(p.ManifestRevision) == 64 &&
		!strings.ContainsAny(p.AUMID+p.FullName, "\x00\r\n/\\ \t") && strings.Count(p.AUMID, "!") == 1
}

// Windows supplies both values from the process token/package APIs, never from
// a window title, executable basename or self-assigned window property.
type processApplicationIdentity struct {
	AUMID    string
	FullName string
}

type packagedDesktopProvider interface {
	TargetApplication(NativeTarget) (processApplicationIdentity, error)
}

func sameInstalledApplication(a, b installedApplication) bool {
	if a.packageIdentity != nil || b.packageIdentity != nil {
		return a.kind == applicationPackaged && b.kind == applicationPackaged &&
			a.packageIdentity != nil && b.packageIdentity != nil && *a.packageIdentity == *b.packageIdentity &&
			a.identity != nil && b.identity != nil && os.SameFile(a.identity, b.identity)
	}
	return (a.kind == "" || a.kind == applicationExecutable) && (b.kind == "" || b.kind == applicationExecutable) &&
		a.identity != nil && b.identity != nil && os.SameFile(a.identity, b.identity)
}

func applicationPrecondition(app installedApplication) string {
	return nativeHash([]any{app.ID, app.kind, app.packageIdentity, app.identity.Size(), app.identity.ModTime()})
}
