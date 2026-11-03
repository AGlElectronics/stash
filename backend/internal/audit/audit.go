// Package audit writes admin/forensics audit log entries inside the same
// transaction as the state change they describe.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// Record persists an audit_log entry. before/after may be any JSON-marshallable
// value (or nil); they are stored as JSONB.
//
// `q` should usually be the *sqlcgen.Queries bound to the active transaction
// so the audit row commits or rolls back atomically with the state change.
func Record(
	ctx context.Context,
	q *sqlcgen.Queries,
	action string,
	entityType string,
	entityID uuid.UUID,
	before, after any,
	note string,
) error {
	beforeJSON, err := marshalForJSONB(before)
	if err != nil {
		return fmt.Errorf("marshal audit before: %w", err)
	}
	afterJSON, err := marshalForJSONB(after)
	if err != nil {
		return fmt.Errorf("marshal audit after: %w", err)
	}

	snap := auth.ActorSnapshotFromContext(ctx)
	var actorSub, actorName *string
	if snap.OIDCSubject != "" {
		actorSub = &snap.OIDCSubject
	}
	if snap.DisplayName != "" {
		actorName = &snap.DisplayName
	}

	_, err = q.InsertAuditLog(ctx, sqlcgen.InsertAuditLogParams{
		ActorUserID:      snap.UserID,
		ActorOidcSubject: actorSub,
		ActorDisplayName: actorName,
		Action:           action,
		EntityType:       entityType,
		EntityID:         entityID,
		BeforeJson:       beforeJSON,
		AfterJson:        afterJSON,
		Note:             note,
	})
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func marshalForJSONB(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}
