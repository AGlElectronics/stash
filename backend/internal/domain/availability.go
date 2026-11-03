package domain

// AvailableQuantity is the count of units that can still be signed out or
// otherwise allocated. It matches the product rule: owned minus signed-out,
// damaged, and lost; negative results clamp to zero (data should not go
// negative in normal operation, but the API stays consistent if it does).
func AvailableQuantity(owned, signedOut, damaged, lost int32) int32 {
	avail := owned - signedOut - damaged - lost
	if avail < 0 {
		return 0
	}
	return avail
}
