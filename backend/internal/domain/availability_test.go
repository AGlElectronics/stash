package domain

import "testing"

func TestAvailableQuantity(t *testing.T) {
	cases := []struct {
		name             string
		owned, signedOut int32
		damaged, lost    int32
		want             int32
	}{
		{
			name:  "nothing committed",
			owned: 10, signedOut: 0, damaged: 0, lost: 0,
			want: 10,
		},
		{
			name:  "signed out reduces available",
			owned: 10, signedOut: 3, damaged: 0, lost: 0,
			want: 7,
		},
		{
			name:  "damaged reduces available",
			owned: 10, signedOut: 0, damaged: 4, lost: 0,
			want: 6,
		},
		{
			name:  "lost reduces available",
			owned: 10, signedOut: 0, damaged: 0, lost: 2,
			want: 8,
		},
		{
			name:  "all four buckets",
			owned: 20, signedOut: 5, damaged: 3, lost: 2,
			want: 10,
		},
		{
			name:  "zero owned",
			owned: 0, signedOut: 0, damaged: 0, lost: 0,
			want: 0,
		},
		{
			name:  "negative raw sum clamps to zero",
			owned: 5, signedOut: 10, damaged: 0, lost: 0,
			want: 0,
		},
		{
			name:  "overcommitted across damaged and lost still clamps",
			owned: 2, signedOut: 1, damaged: 1, lost: 1,
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AvailableQuantity(tc.owned, tc.signedOut, tc.damaged, tc.lost)
			if got != tc.want {
				t.Fatalf("AvailableQuantity(%d,%d,%d,%d)=%d, want %d",
					tc.owned, tc.signedOut, tc.damaged, tc.lost, got, tc.want)
			}
		})
	}
}
