//go:build darwin && cgo

package execution

/*
#include <sys/types.h>
#include <sys/acl.h>
#include <sys/fcntl.h>
#include <sys/stat.h>
#include <membership.h>
#include <unistd.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>

static int offgrid_private_ancestor_acl(const char *path) {
    // acl_get_file reports NULL/ENOENT both for absent ACLs and failed paths.
    // A successful native stat plus an explicit property query distinguishes
    // an ordinary no-ACL directory from unavailable security evidence.
    filesec_t security = filesec_init();
    if (!security) return -1;
    struct stat metadata;
    int present = 0;
    if (lstatx_np(path, &metadata, security) != 0 || filesec_query_property(security, FILESEC_ACL, &present) != 0) {
        filesec_free(security); return -1;
    }
    if (!present) { filesec_free(security); return 0; }
    acl_t acl = NULL;
    int obtained = filesec_get_property(security, FILESEC_ACL, &acl);
    filesec_free(security);
    if (obtained != 0 || !acl) return -1;
    int result = -1;
    uuid_t current, root;
    if (acl_valid(acl) != 0 || mbr_uid_to_uuid(geteuid(), current) != 0 || mbr_uid_to_uuid(0, root) != 0) goto done;
    for (int index = 0; index <= ACL_MAX_ENTRIES; index++) {
        acl_entry_t entry;
        errno = 0;
        if (acl_get_entry(acl, index, &entry) != 0) {
            if (errno == EINVAL) result = 0; // Valid ACL, index reached its end.
            goto done;
        }
        if (index == ACL_MAX_ENTRIES) goto done;
        acl_flagset_t flags;
        acl_tag_t tag;
        if (acl_get_flagset_np(entry, &flags) != 0 || acl_get_tag_type(entry, &tag) != 0) goto done;
        int inherited_only = acl_get_flag_np(flags, ACL_ENTRY_ONLY_INHERIT);
        if (inherited_only < 0) goto done;
        // chmod-based private leaf creation does not remove Darwin ACLs. An
        // inherit-only mutation grant could become effective on that new leaf,
        // so it must also have a trusted qualifier before admitting this tree.
        if (tag == ACL_EXTENDED_DENY) continue;
        if (tag != ACL_EXTENDED_ALLOW) goto done;
        acl_permset_t permissions;
        if (acl_get_permset(entry, &permissions) != 0) goto done;
        const acl_perm_t mutation[] = {ACL_DELETE, ACL_DELETE_CHILD, ACL_WRITE_SECURITY,
            ACL_CHANGE_OWNER, ACL_WRITE_DATA, ACL_APPEND_DATA, ACL_WRITE_ATTRIBUTES, ACL_WRITE_EXTATTRIBUTES};
        int mutable = 0;
        for (unsigned int i = 0; i < sizeof(mutation)/sizeof(mutation[0]); i++) {
            int present = acl_get_perm_np(permissions, mutation[i]);
            if (present < 0) goto done;
            mutable |= present;
        }
        if (!mutable) continue;
        void *qualifier = acl_get_qualifier(entry);
        if (!qualifier) goto done;
        int trusted = memcmp(qualifier, current, sizeof(uuid_t)) == 0 || memcmp(qualifier, root, sizeof(uuid_t)) == 0;
        acl_free(qualifier);
        if (!trusted) goto done;
    }
done:
    acl_free(acl);
    return result;
}
*/
import "C"

import "unsafe"

// Extended Darwin ACL grants are independent of st_mode. Inspect native ACLs;
// unsupported filesystems, unknown qualifiers and unrepresentable entries fail
// closed. Denials are never used to justify a potentially unsafe allow entry.
func checkPrivateExtendedAuthority(path string) error {
	name := C.CString(path)
	defer C.free(unsafe.Pointer(name))
	if C.offgrid_private_ancestor_acl(name) != 0 {
		return ErrAuthority
	}
	return nil
}
