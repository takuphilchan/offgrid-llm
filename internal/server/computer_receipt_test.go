package server

import "testing"

func TestComputerReceiptRequiresConclusiveTypedHostResult(t *testing.T) {
	for _, result := range []string{`{"verified":true}`, `{"actions":[]}`, `{"actions":[{"outcome":"verified"}]}`, `{"actions":[{"action_id":"a","outcome":"uncertain"}]}`, `{"actions":[{"action_id":"a","outcome":"verified","uncertain":true}]}`, `{"actions":[{"action_id":"a","outcome":"failed"}]}`} {
		if recoverableComputerResult(result) {
			t.Fatal("accepted misleading/unknown outcome", result)
		}
	}
	if !recoverableComputerResult(`{"actions":[{"action_id":"a","outcome":"dispatched","uncertain":false}]}`) {
		t.Fatal("rejected action-bound dispatch receipt")
	}
}
