package status

import "testing"

func TestStateVocabularyIsStable(t *testing.T) {
	t.Parallel()
	for _, value := range []State{
		StateNotInstalled,
		StateStopped,
		StateStarting,
		StateRunning,
		StateStopping,
		StateDegraded,
		StateFailed,
		StateUnknown,
	} {
		if !value.Valid() {
			t.Errorf("state %q is not valid", value)
		}
	}
	if State("BROKEN").Valid() {
		t.Fatal("unknown state was accepted")
	}
}
