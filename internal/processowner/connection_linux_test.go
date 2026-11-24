package processowner

import (
	"strings"
	"testing"
)

func TestProviderLinuxSocketInodeExactTupleAndBounds(t *testing.T) {
	server, client := "0100007F:1234", "0100007F:5678"
	row := "0: " + server + " " + client + " 01 0:0 0:0 0 1000 0 12345\n"
	if inode, err := linuxSocketInode([]byte(row), server, client, 1000); err != nil || inode != "12345" {
		t.Fatalf("exact tuple: %q %v", inode, err)
	}
	for _, raw := range []string{row + row, strings.Replace(row, " 1000 ", " 1001 ", 1), strings.Replace(row, "12345", "0", 1)} {
		if _, err := linuxSocketInode([]byte(raw), server, client, 1000); err == nil {
			t.Fatal("accepted ambiguous/unowned inode")
		}
	}
	if inode, err := linuxSocketInode([]byte(row), client, server, 1000); err != nil || inode != "" {
		t.Fatalf("accepted reversed tuple: %q %v", inode, err)
	}
}
