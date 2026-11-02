package agents

import "errors"

var ErrSourceAccess = errors.New("Knowledge access changed. No further work was dispatched; review the source permissions before starting new work.")

func (r *Runner) checkSourceAccess(task *Task) error {
	if r.SourceAccess != nil && !r.SourceAccess(task) {
		task.ErrorCode = "knowledge_unavailable"
		return ErrSourceAccess
	}
	return nil
}
