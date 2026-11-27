package domain_test

import (
	"testing"

	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
)

func TestComputeGroupStatus_AllCombinations(t *testing.T) {
	cases := []struct {
		name     string
		children []domain.BookingStatus
		want     domain.GroupStatus
	}{
		{
			name:     "empty_holding",
			children: nil,
			want:     domain.GroupStatusHolding,
		},
		{
			name:     "all_pending",
			children: []domain.BookingStatus{domain.BookingStatusPending, domain.BookingStatusPending},
			want:     domain.GroupStatusPending,
		},
		{
			name:     "partially_confirmed_pending_and_confirmed",
			children: []domain.BookingStatus{domain.BookingStatusConfirmed, domain.BookingStatusPending, domain.BookingStatusPending},
			want:     domain.GroupStatusPartiallyConfirmed,
		},
		{
			name:     "all_confirmed",
			children: []domain.BookingStatus{domain.BookingStatusConfirmed, domain.BookingStatusConfirmed},
			want:     domain.GroupStatusConfirmed,
		},
		{
			name:     "confirmed_with_checked_in_and_checked_out",
			children: []domain.BookingStatus{domain.BookingStatusConfirmed, domain.BookingStatusCheckedIn, domain.BookingStatusCheckedOut},
			want:     domain.GroupStatusConfirmed,
		},
		{
			name:     "all_checked_out_completed",
			children: []domain.BookingStatus{domain.BookingStatusCheckedOut, domain.BookingStatusCheckedOut},
			want:     domain.GroupStatusCompleted,
		},
		{
			name:     "partially_cancelled_with_confirmed",
			children: []domain.BookingStatus{domain.BookingStatusCancelled, domain.BookingStatusConfirmed, domain.BookingStatusConfirmed},
			want:     domain.GroupStatusPartiallyCancelled,
		},
		{
			name:     "partially_cancelled_with_pending",
			children: []domain.BookingStatus{domain.BookingStatusCancelled, domain.BookingStatusPending},
			want:     domain.GroupStatusPartiallyCancelled,
		},
		{
			name:     "all_cancelled",
			children: []domain.BookingStatus{domain.BookingStatusCancelled, domain.BookingStatusCancelled},
			want:     domain.GroupStatusCancelled,
		},
		{
			name:     "all_expired",
			children: []domain.BookingStatus{domain.BookingStatusExpired, domain.BookingStatusExpired},
			want:     domain.GroupStatusExpired,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.ComputeGroupStatus(tc.children)
			if got != tc.want {
				t.Fatalf("ComputeGroupStatus(%v) = %q, want %q", tc.children, got, tc.want)
			}
		})
	}
}
