package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/events"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

type ItemService struct {
	pool *pgxpool.Pool
}

func NewItemService(pool *pgxpool.Pool) *ItemService {
	return &ItemService{pool: pool}
}

// ----------------------------------------------------------------------------
// Inputs
// ----------------------------------------------------------------------------

type CreateItemInput struct {
	Name                  string
	Description           string
	CategoryID            *uuid.UUID
	CategoryName          string
	ItemType              domain.ItemType
	Size                  string
	QuantityOwned         int32
	Unit                  string
	Condition             string
	Status                string
	LocationID            *uuid.UUID
	LocationName          string
	ParentContainerID     *uuid.UUID
	ParentGroupID         *uuid.UUID
	ManagedByDepartmentID *uuid.UUID
	ManagedByDeptName     string
	NeededByDepartmentID  *uuid.UUID
	NeededByDeptName      string
	SerialNumber          *string
	AssetTag              *string
	QRCode                *string
	Owner                 string
	PurchasePrice         *string
	PurchaseCurrency      string
	PurchaseDate          *time.Time
	ExpiryDate            *time.Time
	Supplier              *string
	PurchaseURL           *string
	ReplacementCost       *string
	MinimumQuantity       int32
	Notes                 string
}

type UpdateItemInput struct {
	Name                  *string
	Description           *string
	CategoryID            *uuid.UUID
	CategoryName          string
	ClearCategory         bool
	ItemType              *string
	Size                  *string
	Unit                  *string
	Condition             *string
	LocationID            *uuid.UUID
	LocationName          string
	ClearLocation         bool
	ParentContainerID     *uuid.UUID
	ClearContainer        bool
	ParentGroupID         *uuid.UUID
	ClearGroup            bool
	ManagedByDepartmentID *uuid.UUID
	ManagedByDeptName     string
	ClearManagedBy        bool
	NeededByDepartmentID  *uuid.UUID
	NeededByDeptName      string
	ClearNeededBy         bool
	SerialNumber          *string
	AssetTag              *string
	QRCode                *string
	Owner                 *string
	PurchasePrice         *string
	ClearPurchasePrice    bool
	PurchaseCurrency      *string
	PurchaseDate          *time.Time
	ClearPurchaseDate     bool
	ExpiryDate            *time.Time
	ClearExpiryDate       bool
	Supplier              *string
	PurchaseURL           *string
	ReplacementCost       *string
	ClearReplacementCost  bool
	MinimumQuantity       *int32
	Notes                 *string
}

type ListItemsInput struct {
	Search                     string
	CategoryID                 *uuid.UUID
	ManagedDeptID              *uuid.UUID
	NeededDeptID               *uuid.UUID
	ParentContainerID          *uuid.UUID
	ParentGroupID              *uuid.UUID
	LocationID                 *uuid.UUID
	IncludeLocationDescendants bool
	ItemType                   string
	Status                     string
	IncludeRetired             bool
	Limit                      int32
	Offset                     int32
}

type ListItemsResult struct {
	Items []sqlcgen.Item
	Total int64
}

type Availability struct {
	QuantityOwned     int32 `json:"quantity_owned"`
	QuantityDamaged   int32 `json:"quantity_damaged"`
	QuantityLost      int32 `json:"quantity_lost"`
	QuantitySignedOut int32 `json:"quantity_signed_out"`
	QuantityAvailable int32 `json:"quantity_available"`
}

// ----------------------------------------------------------------------------
// Reads
// ----------------------------------------------------------------------------

func (s *ItemService) Get(ctx context.Context, id uuid.UUID) (sqlcgen.Item, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetItem(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.Item{}, httperror.NotFound("item")
		}
		return sqlcgen.Item{}, httperror.Internal(err)
	}
	return row, nil
}

func (s *ItemService) List(ctx context.Context, in ListItemsInput) (ListItemsResult, error) {
	if in.Limit <= 0 {
		in.Limit = 50
	}
	if in.Limit > 2000 {
		in.Limit = 2000
	}
	if in.Offset < 0 {
		in.Offset = 0
	}
	q := sqlcgen.New(s.pool)

	includeRetired := in.IncludeRetired
	var includeLocDesc *bool
	if in.IncludeLocationDescendants {
		t := true
		includeLocDesc = &t
	}
	rows, err := q.ListItems(ctx, sqlcgen.ListItemsParams{
		Search:                     optString(in.Search),
		CategoryID:                 in.CategoryID,
		ManagedDeptID:              in.ManagedDeptID,
		NeededDeptID:               in.NeededDeptID,
		ParentContainerID:          in.ParentContainerID,
		ParentGroupID:              in.ParentGroupID,
		LocationID:                 in.LocationID,
		IncludeLocationDescendants: includeLocDesc,
		ItemType:                   optString(in.ItemType),
		Status:                     optString(in.Status),
		IncludeRetired:             &includeRetired,
		Lim:                        in.Limit,
		Off:                        in.Offset,
	})
	if err != nil {
		return ListItemsResult{}, httperror.Internal(err)
	}
	total, err := q.CountItems(ctx, sqlcgen.CountItemsParams{
		Search:                     optString(in.Search),
		CategoryID:                 in.CategoryID,
		ManagedDeptID:              in.ManagedDeptID,
		NeededDeptID:               in.NeededDeptID,
		ParentContainerID:          in.ParentContainerID,
		ParentGroupID:              in.ParentGroupID,
		LocationID:                 in.LocationID,
		IncludeLocationDescendants: includeLocDesc,
		ItemType:                   optString(in.ItemType),
		Status:                     optString(in.Status),
		IncludeRetired:             &includeRetired,
	})
	if err != nil {
		return ListItemsResult{}, httperror.Internal(err)
	}
	return ListItemsResult{Items: rows, Total: total}, nil
}

// Availability returns the current owned/available/signed-out/damaged/lost
// counts for an item. The formula matches doc section 9.
func (s *ItemService) Availability(ctx context.Context, id uuid.UUID) (Availability, error) {
	q := sqlcgen.New(s.pool)
	item, err := q.GetItem(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Availability{}, httperror.NotFound("item")
		}
		return Availability{}, httperror.Internal(err)
	}
	signedOut, err := q.ItemSignedOutAggregate(ctx, id)
	if err != nil {
		return Availability{}, httperror.Internal(err)
	}
	avail := domain.AvailableQuantity(item.QuantityOwned, signedOut, item.QuantityDamaged, item.QuantityLost)
	return Availability{
		QuantityOwned:     item.QuantityOwned,
		QuantityDamaged:   item.QuantityDamaged,
		QuantityLost:      item.QuantityLost,
		QuantitySignedOut: signedOut,
		QuantityAvailable: avail,
	}, nil
}

// ----------------------------------------------------------------------------
// Writes
// ----------------------------------------------------------------------------

func (s *ItemService) Create(ctx context.Context, in CreateItemInput) (sqlcgen.Item, error) {
	sys, err := NewSettingsService(s.pool).Get(ctx)
	if err != nil {
		return sqlcgen.Item{}, err
	}
	var out sqlcgen.Item
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		out, err = s.createInTx(ctx, tx, in, sys)
		return err
	})
	return out, err
}

// createInTx is Create inside the caller's transaction.
func (s *ItemService) createInTx(ctx context.Context, tx pgx.Tx, in CreateItemInput, sys SystemSettings) (sqlcgen.Item, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return sqlcgen.Item{}, httperror.Validation("name is required", nil)
	}
	if !in.ItemType.Valid() {
		return sqlcgen.Item{}, httperror.Validation("item_type must be asset, consumable, container or grouped", nil)
	}
	if in.QuantityOwned < 0 {
		return sqlcgen.Item{}, httperror.Validation("quantity_owned cannot be negative", nil)
	}
	if in.Unit == "" {
		in.Unit = "pcs"
	}
	if in.Condition == "" {
		in.Condition = "good"
	}
	if in.Status == "" {
		in.Status = string(domain.ItemStatusAvailable)
	}
	if in.Owner == "" {
		in.Owner = sys.DefaultOwnerOrEmpty()
	}
	if in.PurchaseCurrency == "" {
		in.PurchaseCurrency = sys.DefaultCurrencyOrEUR()
	}

	var out sqlcgen.Item
	err := func() error {
		qtx := sqlcgen.New(tx)

		categoryID, err := resolveCategoryRefTx(ctx, qtx, in.CategoryID, in.CategoryName)
		if err != nil {
			return err
		}
		locationID, err := resolveLocationRefTx(ctx, qtx, in.LocationID, in.LocationName)
		if err != nil {
			return err
		}
		managedDeptID, err := resolveDepartmentRefTx(ctx, qtx, in.ManagedByDepartmentID, in.ManagedByDeptName)
		if err != nil {
			return err
		}
		neededDeptID, err := resolveDepartmentRefTx(ctx, qtx, in.NeededByDepartmentID, in.NeededByDeptName)
		if err != nil {
			return err
		}

		row, err := qtx.CreateItem(ctx, sqlcgen.CreateItemParams{
			Name:                  in.Name,
			Description:           in.Description,
			CategoryID:            categoryID,
			ItemType:              string(in.ItemType),
			Size:                  in.Size,
			QuantityOwned:         in.QuantityOwned,
			Unit:                  in.Unit,
			Condition:             in.Condition,
			Status:                in.Status,
			LocationID:            locationID,
			ParentContainerID:     in.ParentContainerID,
			ParentGroupID:         in.ParentGroupID,
			ManagedByDepartmentID: managedDeptID,
			NeededByDepartmentID:  neededDeptID,
			SerialNumber:          in.SerialNumber,
			AssetTag:              in.AssetTag,
			QrCode:                in.QRCode,
			Owner:                 in.Owner,
			PurchasePrice:         numericOrNull(in.PurchasePrice),
			PurchaseCurrency:      in.PurchaseCurrency,
			PurchaseDate:          dateOrNull(in.PurchaseDate),
			ExpiryDate:            dateOrNull(in.ExpiryDate),
			Supplier:              in.Supplier,
			PurchaseUrl:           in.PurchaseURL,
			ReplacementCost:       numericOrNull(in.ReplacementCost),
			MinimumQuantity:       in.MinimumQuantity,
			Notes:                 in.Notes,
			CreatedBy:             auth.ActorID(ctx),
		})
		if err != nil {
			return mapPgError(err, "item", "duplicate serial / tag / QR code")
		}
		out = row

		if !hasItemScanCode(in.AssetTag, in.QRCode) {
			prefix, width := sys.BarcodeFormat()
			var assignErr error
			out, _, assignErr = assignItemBarcodeTx(ctx, tx, qtx, row.ID, row, prefix, width)
			if assignErr != nil {
				return assignErr
			}
		}

		if _, err := events.Record(ctx, qtx, row.ID, domain.EventCreated, events.Input{
			Quantity:       in.QuantityOwned,
			ToLocationID:   locationID,
			ToDepartmentID: managedDeptID,
			ConditionAfter: ptr(in.Condition),
			Note:           "item created",
		}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.create", "item", row.ID, nil, row, "")
	}()
	return out, err
}

func (s *ItemService) Update(ctx context.Context, id uuid.UUID, in UpdateItemInput) (sqlcgen.Item, error) {
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		out, err = s.updateInTx(ctx, tx, id, in)
		return err
	})
	return out, err
}

// updateInTx is Update inside the caller's transaction.
func (s *ItemService) updateInTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, in UpdateItemInput) (sqlcgen.Item, error) {
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" {
			return sqlcgen.Item{}, httperror.Validation("name cannot be empty", nil)
		}
		in.Name = &v
	}
	if in.ItemType != nil && !domain.ItemType(*in.ItemType).Valid() {
		return sqlcgen.Item{}, httperror.Validation("item_type must be asset, consumable, container or grouped", nil)
	}

	var out sqlcgen.Item
	err := func() error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}

		categoryID := in.CategoryID
		if !in.ClearCategory && categoryID == nil && strings.TrimSpace(in.CategoryName) != "" {
			categoryID, err = resolveCategoryRefTx(ctx, qtx, nil, in.CategoryName)
			if err != nil {
				return err
			}
		}
		locationID := in.LocationID
		if !in.ClearLocation && locationID == nil && strings.TrimSpace(in.LocationName) != "" {
			locationID, err = resolveLocationRefTx(ctx, qtx, nil, in.LocationName)
			if err != nil {
				return err
			}
		}
		managedDeptID := in.ManagedByDepartmentID
		if !in.ClearManagedBy && managedDeptID == nil && strings.TrimSpace(in.ManagedByDeptName) != "" {
			managedDeptID, err = resolveDepartmentRefTx(ctx, qtx, nil, in.ManagedByDeptName)
			if err != nil {
				return err
			}
		}
		neededDeptID := in.NeededByDepartmentID
		if !in.ClearNeededBy && neededDeptID == nil && strings.TrimSpace(in.NeededByDeptName) != "" {
			neededDeptID, err = resolveDepartmentRefTx(ctx, qtx, nil, in.NeededByDeptName)
			if err != nil {
				return err
			}
		}

		row, err := qtx.UpdateItem(ctx, sqlcgen.UpdateItemParams{
			ID:                id,
			Name:              in.Name,
			Description:       in.Description,
			CategoryID:        resolveOptionalUUID(categoryID, before.CategoryID, in.ClearCategory),
			ItemType:          in.ItemType,
			Size:              in.Size,
			Unit:              in.Unit,
			Condition:         in.Condition,
			LocationID:        resolveOptionalUUID(locationID, before.LocationID, in.ClearLocation),
			ParentContainerID: resolveOptionalUUID(in.ParentContainerID, before.ParentContainerID, in.ClearContainer),
			ParentGroupID:     resolveOptionalUUID(in.ParentGroupID, before.ParentGroupID, in.ClearGroup),
			ManagedDeptID:     resolveOptionalUUID(managedDeptID, before.ManagedByDepartmentID, in.ClearManagedBy),
			NeededDeptID:      resolveOptionalUUID(neededDeptID, before.NeededByDepartmentID, in.ClearNeededBy),
			SerialNumber:      in.SerialNumber,
			AssetTag:          in.AssetTag,
			QrCode:            in.QRCode,
			Owner:             in.Owner,
			PurchasePrice:     resolveOptionalNumeric(in.PurchasePrice, before.PurchasePrice, in.ClearPurchasePrice),
			PurchaseCurrency:  in.PurchaseCurrency,
			PurchaseDate:      resolveOptionalDate(in.PurchaseDate, before.PurchaseDate, in.ClearPurchaseDate),
			ExpiryDate:        resolveOptionalDate(in.ExpiryDate, before.ExpiryDate, in.ClearExpiryDate),
			Supplier:          in.Supplier,
			PurchaseUrl:       in.PurchaseURL,
			ReplacementCost:   resolveOptionalNumeric(in.ReplacementCost, before.ReplacementCost, in.ClearReplacementCost),
			MinimumQuantity:   in.MinimumQuantity,
			Notes:             in.Notes,
		})
		if err != nil {
			return mapPgError(err, "item", "duplicate serial / tag / QR code")
		}
		out = row

		// emit a condition_changed event if the condition column was updated
		if in.Condition != nil && before.Condition != row.Condition {
			if _, err := events.Record(ctx, qtx, id, domain.EventConditionChanged, events.Input{
				ConditionBefore: ptr(before.Condition),
				ConditionAfter:  ptr(row.Condition),
				Note:            "condition changed",
			}); err != nil {
				return err
			}
		}
		return audit.Record(ctx, qtx, "item.update", "item", id, before, row, "")
	}()
	return out, err
}

// ----------------------------------------------------------------------------
// Lifecycle endpoints
// ----------------------------------------------------------------------------

type MoveInput struct {
	ToLocationID   *uuid.UUID
	ClearLocation  bool
	ToContainerID  *uuid.UUID
	ClearContainer bool
	Note           string
}

func (s *ItemService) Move(ctx context.Context, id uuid.UUID, in MoveInput) (sqlcgen.Item, error) {
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		newLocation := resolveOptionalUUID(in.ToLocationID, before.LocationID, in.ClearLocation)
		newContainer := resolveOptionalUUID(in.ToContainerID, before.ParentContainerID, in.ClearContainer)
		row, err := qtx.MoveItem(ctx, sqlcgen.MoveItemParams{
			ID:                id,
			LocationID:        newLocation,
			ParentContainerID: newContainer,
		})
		if err != nil {
			return mapPgError(err, "item", "")
		}
		out = row
		if _, err := events.Record(ctx, qtx, id, domain.EventMoved, events.Input{
			Quantity:       before.QuantityOwned,
			FromLocationID: before.LocationID,
			ToLocationID:   row.LocationID,
			Note:           in.Note,
		}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.move", "item", id, before, row, in.Note)
	})
	return out, err
}

type AdjustInput struct {
	Delta int32
	Note  string
}

func (s *ItemService) AdjustQuantity(ctx context.Context, id uuid.UUID, in AdjustInput) (sqlcgen.Item, error) {
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		out, err = adjustInTx(ctx, tx, id, in)
		return err
	})
	return out, err
}

// adjustInTx is AdjustQuantity inside the caller's transaction.
func adjustInTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, in AdjustInput) (sqlcgen.Item, error) {
	if in.Delta == 0 {
		return sqlcgen.Item{}, httperror.Validation("delta must be non-zero", nil)
	}
	var out sqlcgen.Item
	err := func() error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		if before.QuantityOwned+in.Delta < 0 {
			return httperror.Validation("adjustment would result in negative quantity_owned", nil)
		}
		row, err := qtx.AdjustItemQuantity(ctx, sqlcgen.AdjustItemQuantityParams{
			ID:    id,
			Delta: in.Delta,
		})
		if err != nil {
			return mapPgError(err, "item", "")
		}
		out = row
		if _, err := events.Record(ctx, qtx, id, domain.EventQuantityAdjusted, events.Input{
			Quantity: in.Delta,
			Note:     in.Note,
		}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.adjust", "item", id, before, row, in.Note)
	}()
	return out, err
}

type DamageInput struct {
	Quantity int32
	Note     string
}

func (s *ItemService) MarkDamaged(ctx context.Context, id uuid.UUID, in DamageInput) (sqlcgen.Item, error) {
	if in.Quantity <= 0 {
		return sqlcgen.Item{}, httperror.Validation("quantity must be > 0", nil)
	}
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		if before.QuantityDamaged+in.Quantity > before.QuantityOwned-before.QuantityLost {
			return httperror.Validation("damaged quantity would exceed available units", nil)
		}
		row, err := qtx.SetItemDamagedDelta(ctx, sqlcgen.SetItemDamagedDeltaParams{
			ID:    id,
			Delta: in.Quantity,
		})
		if err != nil {
			return mapPgError(err, "item", "")
		}
		out = row
		if _, err := events.Record(ctx, qtx, id, domain.EventMarkedDamaged, events.Input{
			Quantity:       in.Quantity,
			ConditionAfter: ptr("damaged"),
			Note:           in.Note,
		}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.damage", "item", id, before, row, in.Note)
	})
	return out, err
}

type RepairInput struct {
	Quantity       int32
	Note           string
	RelatedEventID *uuid.UUID
}

func (s *ItemService) Repair(ctx context.Context, id uuid.UUID, in RepairInput) (sqlcgen.Item, error) {
	if in.Quantity <= 0 {
		return sqlcgen.Item{}, httperror.Validation("quantity must be > 0", nil)
	}
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		if in.Quantity > before.QuantityDamaged {
			return httperror.Validation("repair quantity exceeds damaged units", nil)
		}
		row, err := qtx.SetItemDamagedDelta(ctx, sqlcgen.SetItemDamagedDeltaParams{
			ID:    id,
			Delta: -in.Quantity,
		})
		if err != nil {
			return mapPgError(err, "item", "")
		}
		out = row
		if _, err := events.Record(ctx, qtx, id, domain.EventRepaired, events.Input{
			Quantity:        in.Quantity,
			ConditionBefore: ptr("damaged"),
			ConditionAfter:  ptr("good"),
			RelatedEventID:  in.RelatedEventID,
			Note:            in.Note,
		}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.repair", "item", id, before, row, in.Note)
	})
	return out, err
}

type LostInput struct {
	Quantity int32
	Note     string
}

func (s *ItemService) MarkLost(ctx context.Context, id uuid.UUID, in LostInput) (sqlcgen.Item, error) {
	if in.Quantity <= 0 {
		return sqlcgen.Item{}, httperror.Validation("quantity must be > 0", nil)
	}
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		if before.QuantityLost+in.Quantity > before.QuantityOwned {
			return httperror.Validation("lost quantity exceeds owned units", nil)
		}
		row, err := qtx.SetItemLostDelta(ctx, sqlcgen.SetItemLostDeltaParams{
			ID:    id,
			Delta: in.Quantity,
		})
		if err != nil {
			return mapPgError(err, "item", "")
		}
		out = row
		if _, err := events.Record(ctx, qtx, id, domain.EventMarkedLost, events.Input{
			Quantity: in.Quantity,
			Note:     in.Note,
		}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.lose", "item", id, before, row, in.Note)
	})
	return out, err
}

func (s *ItemService) MarkFound(ctx context.Context, id uuid.UUID, in LostInput) (sqlcgen.Item, error) {
	if in.Quantity <= 0 {
		return sqlcgen.Item{}, httperror.Validation("quantity must be > 0", nil)
	}
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		if in.Quantity > before.QuantityLost {
			return httperror.Validation("found quantity exceeds lost units", nil)
		}
		row, err := qtx.SetItemLostDelta(ctx, sqlcgen.SetItemLostDeltaParams{
			ID:    id,
			Delta: -in.Quantity,
		})
		if err != nil {
			return mapPgError(err, "item", "")
		}
		out = row
		if _, err := events.Record(ctx, qtx, id, domain.EventMarkedFound, events.Input{
			Quantity: in.Quantity,
			Note:     in.Note,
		}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.find", "item", id, before, row, in.Note)
	})
	return out, err
}

func (s *ItemService) Retire(ctx context.Context, id uuid.UUID, note string) (sqlcgen.Item, error) {
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		if before.RetiredAt.Valid {
			return httperror.Conflict("item is already retired")
		}
		row, err := qtx.SetItemStatus(ctx, sqlcgen.SetItemStatusParams{
			ID:     id,
			Status: string(domain.ItemStatusRetired),
		})
		if err != nil {
			return mapPgError(err, "item", "")
		}
		out = row
		if _, err := events.Record(ctx, qtx, id, domain.EventRetired, events.Input{Note: note}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.retire", "item", id, before, row, note)
	})
	return out, err
}

func (s *ItemService) Restore(ctx context.Context, id uuid.UUID, note string) (sqlcgen.Item, error) {
	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		if !before.RetiredAt.Valid {
			return httperror.Conflict("item is not retired")
		}
		row, err := qtx.SetItemStatus(ctx, sqlcgen.SetItemStatusParams{
			ID:     id,
			Status: string(domain.ItemStatusAvailable),
		})
		if err != nil {
			return mapPgError(err, "item", "")
		}
		out = row
		if _, err := events.Record(ctx, qtx, id, domain.EventRestored, events.Input{Note: note}); err != nil {
			return err
		}
		return audit.Record(ctx, qtx, "item.restore", "item", id, before, row, note)
	})
	return out, err
}

// ----------------------------------------------------------------------------
// History
// ----------------------------------------------------------------------------

type ListHistoryInput struct {
	EventType string
	SignoutID *uuid.UUID
	Since     *time.Time
	Limit     int32
	Offset    int32
}

type ListHistoryResult struct {
	Events []sqlcgen.ItemEvent
	Total  int64
}

func (s *ItemService) ListHistory(ctx context.Context, id uuid.UUID, in ListHistoryInput) (ListHistoryResult, error) {
	if in.Limit <= 0 {
		in.Limit = 100
	}
	if in.Limit > 500 {
		in.Limit = 500
	}
	q := sqlcgen.New(s.pool)
	rows, err := q.ListItemEvents(ctx, sqlcgen.ListItemEventsParams{
		ItemID:    id,
		EventType: optString(in.EventType),
		SignoutID: in.SignoutID,
		Since:     tsOrNull(in.Since),
		Lim:       in.Limit,
		Off:       in.Offset,
	})
	if err != nil {
		return ListHistoryResult{}, httperror.Internal(err)
	}
	total, err := q.CountItemEvents(ctx, sqlcgen.CountItemEventsParams{
		ItemID:    id,
		EventType: optString(in.EventType),
		SignoutID: in.SignoutID,
		Since:     tsOrNull(in.Since),
	})
	if err != nil {
		return ListHistoryResult{}, httperror.Internal(err)
	}
	return ListHistoryResult{Events: rows, Total: total}, nil
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptr[T any](v T) *T { return &v }

func resolveOptionalUUID(in *uuid.UUID, current *uuid.UUID, clear bool) *uuid.UUID {
	if clear {
		return nil
	}
	if in != nil {
		v := *in
		return &v
	}
	return current
}

func resolveOptionalNumeric(in *string, current pgtype.Numeric, clear bool) pgtype.Numeric {
	if clear {
		return pgtype.Numeric{}
	}
	if in != nil {
		var n pgtype.Numeric
		_ = n.Scan(*in)
		return n
	}
	return current
}

func resolveOptionalDate(in *time.Time, current pgtype.Date, clear bool) pgtype.Date {
	if clear {
		return pgtype.Date{}
	}
	if in != nil {
		return pgtype.Date{Time: *in, Valid: true}
	}
	return current
}

func numericOrNull(s *string) pgtype.Numeric {
	if s == nil || *s == "" {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	_ = n.Scan(*s)
	return n
}

func dateOrNull(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func tsOrNull(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
