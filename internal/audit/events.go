package audit

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func NewID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return prefix + "-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "-" + hex.EncodeToString(buffer)
}

func Event(eventType, actor string, role domain.Role, before, after domain.CaseStatus, version int64, now time.Time, details map[string]any) domain.AuditEvent {
	return domain.AuditEvent{
		ID: NewID("evt"), Type: eventType, Actor: actor, Role: role, BeforeStatus: before,
		AfterStatus: after, Version: version, OccurredAt: now.UTC(), Details: details,
	}
}
