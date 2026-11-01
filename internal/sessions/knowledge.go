package sessions

import "github.com/takuphilchan/offgrid-llm/pkg/api"

// KnowledgePassage is the bounded reference text actually supplied to inference,
// not a claim that the model's interpretation or citation is correct.
type KnowledgePassage = api.KnowledgePassage
