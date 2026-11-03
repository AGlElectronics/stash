// Package events writes per-item lifecycle events used by the user-facing
// item history timeline. Each call should run inside the same transaction
// as the state change it describes so the event row commits or rolls back
// atomically.
package events

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// Input groups optional fields for an item event.
type Input struct {
	Quantity         int32
	FromLocationID   *uuid.UUID
	ToLocationID     *uuid.UUID
	FromDepartmentID *uuid.UUID
	ToDepartmentID   *uuid.UUID
	RelatedSignoutID *uuid.UUID
	RelatedEventID   *uuid.UUID
	RelatedFileID    *uuid.UUID
	ConditionBefore  *string
	ConditionAfter   *string
	Note             string
}

// Record persists an item event. Returns the inserted row id (useful when
// later events need to reference it via related_event_id, e.g. repaired ->
// the original marked_damaged event).
func Record(
	ctx context.Context,
	q *sqlcgen.Queries,
	itemID uuid.UUID,
	eventType domain.ItemEventType,
	in Input,
) (uuid.UUID, error) {
	snap := auth.ActorSnapshotFromContext(ctx)
	var actorSub, actorName *string
	if snap.OIDCSubject != "" {
		actorSub = &snap.OIDCSubject
	}
	if snap.DisplayName != "" {
		actorName = &snap.DisplayName
	}

	row, err := q.InsertItemEvent(ctx, sqlcgen.InsertItemEventParams{
		ItemID:           itemID,
		EventType:        string(eventType),
		Quantity:         in.Quantity,
		FromLocationID:   in.FromLocationID,
		ToLocationID:     in.ToLocationID,
		FromDepartmentID: in.FromDepartmentID,
		ToDepartmentID:   in.ToDepartmentID,
		RelatedSignoutID: in.RelatedSignoutID,
		RelatedEventID:   in.RelatedEventID,
		RelatedFileID:    in.RelatedFileID,
		ConditionBefore:  in.ConditionBefore,
		ConditionAfter:   in.ConditionAfter,
		ActorUserID:      snap.UserID,
		ActorOidcSubject: actorSub,
		ActorDisplayName: actorName,
		Note:             in.Note,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert item event: %w", err)
	}
	return row.ID, nil
}
