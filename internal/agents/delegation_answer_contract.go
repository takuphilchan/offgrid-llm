package agents

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var (
	requestedBriefingWords = regexp.MustCompile(`(?i)\b(\d{2,4})[\s-]+word\s+(?:briefing|report|summary|response)\b`)
	suppliedReviewFigures  = regexp.MustCompile(`(?i)\b(\d+)\s+of\s+(\d+)\s+tasks\s+completed\s*,?\s*(?:and\s+)?(\d+)\s+of\s+those\s+(\d+)\s+passed\s+review\b`)
	answerPercent          = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)
)

// Check only mechanical requirements explicitly present in the user's root
// request. These checks do not judge the quality of a briefing or trust child
// prose as evidence. A failed answer is corrected without dispatching tools.
func delegatedAnswerFeedback(task *Task, answer string) string {
	if task.ParentID != "" || len(task.ChildHistory) == 0 {
		return ""
	}
	var missing []string
	figures := suppliedReviewFigures.FindAllStringSubmatch(task.Prompt, -1)
	// This contract is for a supplied-figures briefing, where every required
	// number can be checked without interpreting specialist prose. Other
	// briefings need their own reviewed contract and must keep the existing
	// completion behavior.
	if len(figures) < 2 || !strings.Contains(strings.ToLower(task.Prompt), "rate") {
		return ""
	}
	if match := requestedBriefingWords.FindStringSubmatch(task.Prompt); len(match) == 2 {
		requested, _ := strconv.Atoi(match[1])
		actual := len(strings.Fields(answer))
		minimum := int(math.Ceil(float64(requested) * 0.9))
		maximum := int(math.Floor(float64(requested) * 1.1))
		if actual < minimum || actual > maximum {
			missing = append(missing, fmt.Sprintf("the requested %d-word length (current answer: %d words; accepted range: %d–%d)", requested, actual, minimum, maximum))
		}
	}
	{
		var present []float64
		for _, percent := range answerPercent.FindAllStringSubmatch(answer, -1) {
			value, err := strconv.ParseFloat(percent[1], 64)
			if err == nil {
				present = append(present, value)
			}
		}
		for _, figure := range figures {
			completed, _ := strconv.ParseFloat(figure[1], 64)
			total, _ := strconv.ParseFloat(figure[2], 64)
			passed, _ := strconv.ParseFloat(figure[3], 64)
			passedOf, _ := strconv.ParseFloat(figure[4], 64)
			if completed == 0 || total == 0 || passedOf != completed {
				continue
			}
			for _, expected := range []float64{100 * completed / total, 100 * passed / completed} {
				found := false
				for _, value := range present {
					if math.Abs(value-expected) <= 0.2 {
						found = true
						break
					}
				}
				if !found {
					missing = append(missing, fmt.Sprintf("a requested rate of %.1f%% derived from the supplied figures", expected))
				}
			}
		}
	}
	return strings.Join(missing, "; ")
}

func (r *Runner) repairDelegatedAnswer(task *Task, answer string) (bool, error) {
	feedback := delegatedAnswerFeedback(task, answer)
	if feedback == "" {
		return false, nil
	}
	cp := task.Checkpoint
	cp.DelegationAnswerRepairs++
	if cp.DelegationAnswerRepairs > 2 {
		task.ErrorCode = "delegation_answer_incomplete"
		_, err := r.fail(task, "The specialist results are saved, but the final answer still omits required details: "+feedback+". Review the completed subtasks; no work was replayed.")
		return true, err
	}
	message := "The proposed final answer is incomplete: " + feedback + ". Revise the answer using the saved specialist results and the original user request. Include the requested numerical rates and length. Do not call tools, repeat delegation, invent evidence, or claim work that was not completed."
	task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "delegation_answer_rejected", Content: message, Timestamp: time.Now().UTC()})
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "system", Content: message})
	return true, r.persist(task)
}
