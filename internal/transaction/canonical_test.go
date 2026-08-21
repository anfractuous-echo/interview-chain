package transaction

import "testing"

func TestCanonicalPayload(t *testing.T) {
	t.Parallel()
	tx := Signed{PaymentID: "pay-1", From: "alice", To: "bob", AmountUnits: 12_340_000}
	payload, err := CanonicalPayload(tx)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"payment_id":"pay-1","from":"alice","to":"bob","amount_units":12340000}`
	if string(payload) != want {
		t.Fatalf("canonical payload = %s, want %s", payload, want)
	}
}
