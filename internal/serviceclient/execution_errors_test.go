package serviceclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecutionAndProfileErrorsKeepSafeRecoveryCodes(t *testing.T) {
	for _, code := range []string{"execution_unavailable", "profile_model_mismatch"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(422)
			w.Write([]byte(`{"error":{"code":"` + code + `","message":"PRIVATE-INJECTED-TEXT"}}`))
		}))
		client, err := New(server.URL, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		err = client.JSON(context.Background(), "POST", "/api/v2/jobs", map[string]string{}, nil)
		server.Close()
		var safe *Error
		if !errors.As(err, &safe) || safe.Code != code || strings.Contains(safe.Message, "PRIVATE") || safe.Retryable {
			t.Fatal("unsafe or lossy failure", err)
		}
	}
}
