package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A name and unguessable owner are durable BEFORE create; an acknowledged ID is
// durable BEFORE start. Recovery only stops/removes these objects, never starts
// them or adopts their output as proof of the original operation's success.
type containerOwnership struct {
	Operation, Daemon, Name, Owner, ID string
}

func cleanupStoppedWorker(config Context, directory string) error {
	if !validBackend(config) || config.Backend != "isolated_linux" || !filepath.IsAbs(config.Root) || !filepath.IsAbs(directory) {
		return ErrAuthority
	}
	// Root deletion must not strand an owned container. Never use a replacement
	// workspace as authority: cleanup only reads the original private journal.
	if _, err := os.Lstat(config.Root); os.IsNotExist(err) {
		rel, err := filepath.Rel(config.Root, directory)
		if err != nil || !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return ErrAuthority
		}
	} else if err != nil || PrivateStateOutsideRoot(config.Root, directory) != nil {
		return ErrAuthority
	}
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return nil
	}
	if CheckPrivatePath(directory) != nil {
		return ErrAuthority
	}
	journal, err := OpenJournal(directory)
	if err != nil {
		return err
	}
	defer journal.Close()
	all, err := journal.ownedContainers(context.Background())
	if err != nil || len(all) == 0 {
		return err
	}
	client, err := localIsolationClient(config.Isolation.Socket)
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	i := &LinuxIsolation{policy: config.Isolation, root: config.Root, write: config.Permissions.Write, client: client}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var daemon struct{ ID string }
	if err = i.get(ctx, "/info", &daemon); err != nil {
		return err
	}
	if !validIdentity(daemon.ID) {
		return ErrAuthority
	}
	i.daemonID = daemon.ID
	return i.recover(ctx, journal)
}

func (j *Journal) claimContainer(ctx context.Context, r Request, c containerOwnership) error {
	result, err := j.db.ExecContext(ctx, `INSERT INTO execution_containers(operation,daemon,name,owner,container) SELECT id,?,?,?,'' FROM execution_operations WHERE id=? AND digest=? AND state='claimed' AND (SELECT COUNT(*) FROM execution_containers)<128`, c.Daemon, c.Name, c.Owner, r.OperationID, digest(r.Payload()))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func (j *Journal) identifyContainer(ctx context.Context, c containerOwnership) error {
	r, err := j.db.ExecContext(ctx, `UPDATE execution_containers SET container=? WHERE operation=? AND daemon=? AND name=? AND owner=? AND container=''`, c.ID, c.Operation, c.Daemon, c.Name, c.Owner)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func (j *Journal) forgetContainer(ctx context.Context, c containerOwnership) error {
	_, err := j.db.ExecContext(ctx, `DELETE FROM execution_containers WHERE operation=? AND daemon=? AND name=? AND owner=?`, c.Operation, c.Daemon, c.Name, c.Owner)
	return err
}

func (j *Journal) ownedContainers(ctx context.Context) ([]containerOwnership, error) {
	rows, err := j.db.QueryContext(ctx, `SELECT operation,daemon,name,owner,container FROM execution_containers ORDER BY operation`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []containerOwnership{}
	for rows.Next() {
		var c containerOwnership
		if err = rows.Scan(&c.Operation, &c.Daemon, &c.Name, &c.Owner, &c.ID); err != nil {
			return nil, err
		}
		result = append(result, c)
		if len(result) > 128 {
			return nil, ErrLimit
		}
	}
	return result, rows.Err()
}

func (i *LinuxIsolation) recover(ctx context.Context, journal *Journal) error {
	all, err := journal.ownedContainers(ctx)
	if err != nil {
		return err
	}
	for _, c := range all {
		if err = i.cleanup(ctx, c); err != nil {
			return errors.Join(ErrHostUnavailable, err)
		}
		if err = journal.forgetContainer(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
