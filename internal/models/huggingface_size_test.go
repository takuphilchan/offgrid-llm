package models

import "testing"

func TestSearchSizeDoesNotTreatParametersAsBytes(t *testing.T) {
	model := HFModel{SafeTensors: &HFSafeTensors{Total: 938008576}}
	if got := modelTotalSize(model); got != 0 {
		t.Fatalf("parameter count exposed as bytes: %d", got)
	}
	model.Siblings = []HFFile{{Size: 1876017152}, {Size: 100}}
	if got := modelTotalSize(model); got != 1876017252 {
		t.Fatalf("wrong artifact bytes: %d", got)
	}
}
