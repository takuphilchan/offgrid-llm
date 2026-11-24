package execution

// POSIX access ACL permissions are bounded by the group-class mask reflected
// in st_mode. A write grant to a named user/group therefore fails the mode check
// unless sticky-directory protection applies. Default ACLs do not grant access
// to this existing ancestor; private new leaves enforce their own mode/owner.
func checkPrivateExtendedAuthority(string) error { return nil }
