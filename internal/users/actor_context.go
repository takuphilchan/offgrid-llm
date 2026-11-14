package users

import "context"

// ContextForActor is for service-owned durable work whose actor identity was
// authenticated at acceptance. It is not an HTTP authentication mechanism.
// Snapshot only identity and role under the store lock, without credentials or
// mutable metadata. Dispatch must still recheck current resource permissions.
func (s *UserStore) ContextForActor(ctx context.Context, id string, permission Permission) (context.Context, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok || user.Disabled || !user.HasPermission(permission) {
		return ctx, false
	}
	identity := &User{ID: user.ID, Username: user.Username, Role: user.Role}
	return context.WithValue(ctx, userContextKey, identity), true
}
