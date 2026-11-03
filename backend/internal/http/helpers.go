package http

import (
	"strconv"

	"github.com/google/uuid"
)

func itoa32(v int32) string { return strconv.FormatInt(int64(v), 10) }

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func uuidStr(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}
