package phase7

import (
	"encoding/json"
	"strings"
	"testing"
)

// A sync that produced no clusters was rejected outright with
// "milestoneClusters must be an array", because a nil slice marshals as null
// and the service requires an array. Empty and absent are not the same thing
// on the wire.
func TestEmptyCollectionsMarshalAsArraysNotNull(t *testing.T) {
	body, err := json.Marshal(Assemble(AssemblyInput{}))
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	for _, field := range []string{"milestoneClusters", "shas", "timestamps", "additions", "deletions", "filesChanged", "categories"} {
		if strings.Contains(string(body), `"`+field+`":null`) {
			t.Errorf("%s encoded as null; the service rejects the payload unless it is an array", field)
		}
		if !strings.Contains(string(body), `"`+field+`":[`) {
			t.Errorf("%s is not encoded as an array: %s", field, body)
		}
	}
}
