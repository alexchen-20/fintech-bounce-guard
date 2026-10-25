package main

import "testing"

func TestDecidePaymentRisk(t *testing.T) {
	tests := []struct {
		name       string
		event      PaymentEvent
		suppressed bool
		action     string
	}{
		{"hard bounce", PaymentEvent{PaymentID: "pay-1", Bounce: true}, false, "hold_payment"},
		{"known suppression", PaymentEvent{PaymentID: "pay-2"}, true, "hold_payment"},
		{"deliverable", PaymentEvent{PaymentID: "pay-3"}, false, "continue_payment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decide(tt.event, tt.suppressed)
			if got.Action != tt.action {
				t.Fatalf("action = %q, want %q", got.Action, tt.action)
			}
		})
	}
}
