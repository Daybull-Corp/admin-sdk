package module

import "testing"

func TestValidateEvents(t *testing.T) {
	ok := Manifest{ID: "orders", Events: []Event{
		{Key: "order_ready", Label: "Hazır", Template: "{no} hazır", Action: "orders.status", To: "ready", VarsSQL: "SELECT 1"},
		{Key: "order_due", Label: "Yaklaşıyor", Template: "x", Reminder: true, VarsSQL: "SELECT 1", DueSQL: "SELECT 1"},
	}}
	if err := ValidateEvents(ok); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []Event{
		{Key: "Order-Ready", Label: "x", Template: "x", Action: "a", VarsSQL: "s"},
		{Key: "a", Label: "x", Template: "x", VarsSQL: "s"},                 // anlık ama Action yok
		{Key: "a", Label: "x", Template: "x", Reminder: true, VarsSQL: "s"}, // zamanlı ama DueSQL yok
		{Key: "a", Label: "x", Template: "x", Action: "a"},                  // VarsSQL yok
	} {
		if ValidateEvents(Manifest{ID: "m", Events: []Event{bad}}) == nil {
			t.Fatalf("geçersiz kabul edildi: %+v", bad)
		}
	}
	if ValidateEvents(Manifest{ID: "m", Events: []Event{ok.Events[0], ok.Events[0]}}) == nil {
		t.Fatal("tekrar eden anahtar kabul edildi")
	}
}
