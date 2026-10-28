package execution

import (
	"reflect"
	"strings"
	"testing"
)

func TestRepeatedRoutineProgramIsOnlyAProposalDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		program string
		args    []string
		want    bool
	}{
		{"go", []string{"go", "test", "."}, true},
		{"go.exe", []string{"go.exe", "test", "./..."}, true},
		{"npm", []string{"npm", "run", "build"}, true},
		{"git", []string{"git", "status", "--short"}, true},
		{"go", []string{"test", "."}, false},
		{"go", []string{"go", "test", "-exec=other"}, false},
		{"go", []string{"go", "test", "../other"}, false},
		{"npm", []string{"npm", "install"}, false},
		{"echo", []string{"echo", "hello"}, false},
		{"sh", []string{"sh", "-c", "anything"}, false},
		{"go", []string{"go"}, false},
	} {
		t.Run(tc.program+"/"+strings.Join(tc.args, "_"), func(t *testing.T) {
			spec := ProcessSpec{Program: tc.program, Arguments: append([]string{}, tc.args...)}
			if got := RepeatsRoutineProgram(spec); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if !reflect.DeepEqual(spec.Arguments, tc.args) {
				t.Fatal("diagnostic rewrote arguments")
			}
			if tc.want && RoutineProcess(spec) {
				t.Fatal("diagnostic granted command authority")
			}
		})
	}
	if RepeatsRoutineProgram(ProcessSpec{Program: "go", Arguments: []string{"go", "test", "."}, TimeoutSeconds: 3601}) {
		t.Fatal("invalid bounds classified as repairable")
	}
}
