package http

import (
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/files"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

type itemHandler struct {
	svc   *service.ItemService
	files *files.Service
}

type itemDTO struct {
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	Description           string  `json:"description"`
	CategoryID            *string `json:"category_id,omitempty"`
	ItemType              string  `json:"item_type"`
	Size                  string  `json:"size"`
	QuantityOwned         int32   `json:"quantity_owned"`
	QuantityDamaged       int32   `json:"quantity_damaged"`
	QuantityLost          int32   `json:"quantity_lost"`
	Unit                  string  `json:"unit"`
	Condition             string  `json:"condition"`
	Status                string  `json:"status"`
	LocationID            *string `json:"location_id,omitempty"`
	ParentContainerID     *string `json:"parent_container_id,omitempty"`
	ParentGroupID         *string `json:"parent_group_id,omitempty"`
	ManagedByDepartmentID *string `json:"managed_by_department_id,omitempty"`
	NeededByDepartmentID  *string `json:"needed_by_department_id,omitempty"`
	SerialNumber          *string `json:"serial_number,omitempty"`
	AssetTag              *string `json:"asset_tag,omitempty"`
	QRCode                *string `json:"qr_code,omitempty"`
	Owner                 string  `json:"owner"`
	PurchasePrice         string  `json:"purchase_price,omitempty"`
	PurchaseCurrency      string  `json:"purchase_currency"`
	PurchaseDate          string  `json:"purchase_date,omitempty"`
	ExpiryDate            string  `json:"expiry_date,omitempty"`
	Supplier              *string `json:"supplier,omitempty"`
	PurchaseURL           *string `json:"purchase_url,omitempty"`
	ReplacementCost       string  `json:"replacement_cost,omitempty"`
	MinimumQuantity       int32   `json:"minimum_quantity"`
	PrimaryImageFileID    *string `json:"primary_image_file_id,omitempty"`
	Notes                 string  `json:"notes"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
	RetiredAt             string  `json:"retired_at,omitempty"`
}

func toItemDTO(it sqlcgen.Item) itemDTO {
	dto := itemDTO{
		ID:               it.ID.String(),
		Name:             it.Name,
		Description:      it.Description,
		ItemType:         it.ItemType,
		Size:             it.Size,
		QuantityOwned:    it.QuantityOwned,
		QuantityDamaged:  it.QuantityDamaged,
		QuantityLost:     it.QuantityLost,
		Unit:             it.Unit,
		Condition:        it.Condition,
		Status:           it.Status,
		Owner:            it.Owner,
		PurchasePrice:    numericString(it.PurchasePrice),
		PurchaseCurrency: it.PurchaseCurrency,
		PurchaseDate:     dateString(it.PurchaseDate),
		ExpiryDate:       dateString(it.ExpiryDate),
		ReplacementCost:  numericString(it.ReplacementCost),
		MinimumQuantity:  it.MinimumQuantity,
		Notes:            it.Notes,
		CreatedAt:        timestampString(it.CreatedAt),
		UpdatedAt:        timestampString(it.UpdatedAt),
		RetiredAt:        timestampString(it.RetiredAt),
	}
	dto.CategoryID = uuidToStrPtr(it.CategoryID)
	dto.LocationID = uuidToStrPtr(it.LocationID)
	dto.ParentContainerID = uuidToStrPtr(it.ParentContainerID)
	dto.ParentGroupID = uuidToStrPtr(it.ParentGroupID)
	dto.ManagedByDepartmentID = uuidToStrPtr(it.ManagedByDepartmentID)
	dto.NeededByDepartmentID = uuidToStrPtr(it.NeededByDepartmentID)
	dto.PrimaryImageFileID = uuidToStrPtr(it.PrimaryImageFileID)
	dto.SerialNumber = it.SerialNumber
	dto.AssetTag = it.AssetTag
	dto.QRCode = it.QrCode
	dto.Supplier = it.Supplier
	dto.PurchaseURL = it.PurchaseUrl
	return dto
}

func uuidToStrPtr(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

func (h *itemHandler) routes(r chi.Router) {
	view := r.With(require(domain.PermItemView))
	view.Get("/", h.list)
	r.With(require(domain.PermItemCreate)).Post("/", h.create)
	// Static prefixes must be registered before "/{id}" so chi's tree
	// resolves them deterministically.
	view.Get("/by-barcode/{code}", h.lookupByBarcode)
	r.With(requireAnyOf(domain.PermItemCreate, domain.PermItemEdit)).Get("/barcodes/peek-next", h.peekNextBarcode)
	view.Get("/{id}", h.get)
	r.With(require(domain.PermItemEdit)).Patch("/{id}", h.update)
	r.With(require(domain.PermItemEdit)).Post("/{id}/assign-barcode", h.assignBarcode)
	r.With(require(domain.PermItemEdit)).Post("/{id}/set-barcode", h.setBarcode)

	view.Get("/{id}/availability", h.availability)
	view.Get("/{id}/history", h.history)

	r.With(require(domain.PermItemMove)).Post("/{id}/move", h.move)
	r.With(require(domain.PermItemAdjust)).Post("/{id}/adjust-quantity", h.adjust)
	r.With(require(domain.PermItemDamage)).Post("/{id}/damage", h.damage)
	r.With(require(domain.PermItemDamage)).Post("/{id}/repair", h.repair)
	r.With(require(domain.PermItemLose)).Post("/{id}/lost", h.lost)
	r.With(require(domain.PermItemLose)).Post("/{id}/found", h.found)
	r.With(require(domain.PermItemRetire)).Post("/{id}/retire", h.retire)
	r.With(require(domain.PermItemRetire)).Post("/{id}/restore", h.restore)

	r.With(require(domain.PermItemEdit)).Post("/{id}/primary-image", h.setPrimaryImage)
	r.With(require(domain.PermItemEdit)).Delete("/{id}/primary-image", h.clearPrimaryImage)
}

type setPrimaryImageReq struct {
	FileID string `json:"file_id"`
}

func (h *itemHandler) setPrimaryImage(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req setPrimaryImageReq
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	fileID, perr := uuid.Parse(req.FileID)
	if perr != nil {
		httperror.Render(w, r, httperror.BadRequest("file_id must be a UUID"))
		return
	}
	if err := h.files.SetItemPrimaryImage(r.Context(), id, &fileID); err != nil {
		httperror.Render(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *itemHandler) clearPrimaryImage(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	if err := h.files.SetItemPrimaryImage(r.Context(), id, nil); err != nil {
		httperror.Render(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *itemHandler) list(w http.ResponseWriter, r *http.Request) {
	categoryID, err := queryUUID(r, "category_id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	deptID, err := queryUUID(r, "managed_dept_id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	neededDeptID, err := queryUUID(r, "needed_dept_id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	parentContainerID, err := queryUUID(r, "parent_container_id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	parentGroupID, err := queryUUID(r, "parent_group_id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	locationID, err := queryUUID(r, "location_id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	res, err := h.svc.List(r.Context(), service.ListItemsInput{
		Search:                     queryString(r, "q"),
		CategoryID:                 categoryID,
		ManagedDeptID:              deptID,
		NeededDeptID:               neededDeptID,
		ParentContainerID:          parentContainerID,
		ParentGroupID:              parentGroupID,
		LocationID:                 locationID,
		IncludeLocationDescendants: queryBool(r, "include_sublocations"),
		ItemType:                   queryString(r, "item_type"),
		Status:                     queryString(r, "status"),
		IncludeRetired:             queryBool(r, "include_retired"),
		Limit:                      queryInt(r, "limit", 50),
		Offset:                     queryInt(r, "offset", 0),
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out := make([]itemDTO, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, toItemDTO(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": res.Total})
}

func (h *itemHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

// lookupByBarcode resolves an item by asset_tag (preferred), qr_code or
// serial_number. The code comes URL-encoded in the path so we decode it.
func (h *itemHandler) lookupByBarcode(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "code")
	code, err := url.PathUnescape(raw)
	if err != nil {
		httperror.Render(w, r, httperror.BadRequest("invalid barcode encoding"))
		return
	}
	row, err := h.svc.LookupByCode(r.Context(), code)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

// peekNextBarcode returns the next AW code that would be issued without
// consuming the sequence. Useful for UI previews like "next available: ST000043".
func (h *itemHandler) peekNextBarcode(w http.ResponseWriter, r *http.Request) {
	code, err := h.svc.PeekNextAWBarcode(r.Context())
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"next": code, "format": "AW######"})
}

// assignBarcode generates a fresh AW code and writes it to asset_tag.
func (h *itemHandler) assignBarcode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, code, err := h.svc.AssignAWBarcode(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"item": toItemDTO(row),
		"code": code,
	})
}

// setBarcode stores a caller-provided code (e.g. pre-printed sticker) on the
// item. Used by the "Claim pre-printed barcode" flow on the scan page.
func (h *itemHandler) setBarcode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.SetItemBarcode(r.Context(), id, req.Code)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

type itemCreateRequest struct {
	Name                    string     `json:"name"`
	Description             string     `json:"description,omitempty"`
	CategoryID              *uuid.UUID `json:"category_id,omitempty"`
	CategoryName            string     `json:"category_name,omitempty"`
	ItemType                string     `json:"item_type"`
	Size                    string     `json:"size,omitempty"`
	QuantityOwned           int32      `json:"quantity_owned"`
	Unit                    string     `json:"unit,omitempty"`
	Condition               string     `json:"condition,omitempty"`
	Status                  string     `json:"status,omitempty"`
	LocationID              *uuid.UUID `json:"location_id,omitempty"`
	LocationName            string     `json:"location_name,omitempty"`
	ParentContainerID       *uuid.UUID `json:"parent_container_id,omitempty"`
	ParentGroupID           *uuid.UUID `json:"parent_group_id,omitempty"`
	ManagedByDepartmentID   *uuid.UUID `json:"managed_by_department_id,omitempty"`
	ManagedByDepartmentName string     `json:"managed_by_department_name,omitempty"`
	NeededByDepartmentID    *uuid.UUID `json:"needed_by_department_id,omitempty"`
	NeededByDepartmentName  string     `json:"needed_by_department_name,omitempty"`
	SerialNumber            *string    `json:"serial_number,omitempty"`
	AssetTag                *string    `json:"asset_tag,omitempty"`
	QRCode                  *string    `json:"qr_code,omitempty"`
	Owner                   string     `json:"owner,omitempty"`
	PurchasePrice           *string    `json:"purchase_price,omitempty"`
	PurchaseCurrency        string     `json:"purchase_currency,omitempty"`
	PurchaseDate            *string    `json:"purchase_date,omitempty"`
	ExpiryDate              *string    `json:"expiry_date,omitempty"`
	Supplier                *string    `json:"supplier,omitempty"`
	PurchaseURL             *string    `json:"purchase_url,omitempty"`
	ReplacementCost         *string    `json:"replacement_cost,omitempty"`
	MinimumQuantity         int32      `json:"minimum_quantity,omitempty"`
	Notes                   string     `json:"notes,omitempty"`
}

func parseDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil, httperror.BadRequest("date must be YYYY-MM-DD")
	}
	return &t, nil
}

func (h *itemHandler) create(w http.ResponseWriter, r *http.Request) {
	var req itemCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	purchaseDate, err := parseDate(req.PurchaseDate)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	expiryDate, err := parseDate(req.ExpiryDate)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Create(r.Context(), service.CreateItemInput{
		Name:                  req.Name,
		Description:           req.Description,
		CategoryID:            req.CategoryID,
		CategoryName:          req.CategoryName,
		ItemType:              domain.ItemType(req.ItemType),
		Size:                  req.Size,
		QuantityOwned:         req.QuantityOwned,
		Unit:                  req.Unit,
		Condition:             req.Condition,
		Status:                req.Status,
		LocationID:            req.LocationID,
		LocationName:          req.LocationName,
		ParentContainerID:     req.ParentContainerID,
		ParentGroupID:         req.ParentGroupID,
		ManagedByDepartmentID: req.ManagedByDepartmentID,
		ManagedByDeptName:     req.ManagedByDepartmentName,
		NeededByDepartmentID:  req.NeededByDepartmentID,
		NeededByDeptName:      req.NeededByDepartmentName,
		SerialNumber:          req.SerialNumber,
		AssetTag:              req.AssetTag,
		QRCode:                req.QRCode,
		Owner:                 req.Owner,
		PurchasePrice:         req.PurchasePrice,
		PurchaseCurrency:      req.PurchaseCurrency,
		PurchaseDate:          purchaseDate,
		ExpiryDate:            expiryDate,
		Supplier:              req.Supplier,
		PurchaseURL:           req.PurchaseURL,
		ReplacementCost:       req.ReplacementCost,
		MinimumQuantity:       req.MinimumQuantity,
		Notes:                 req.Notes,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toItemDTO(row))
}

type itemUpdateRequest struct {
	Name                    *string    `json:"name,omitempty"`
	Description             *string    `json:"description,omitempty"`
	CategoryID              *uuid.UUID `json:"category_id,omitempty"`
	CategoryName            string     `json:"category_name,omitempty"`
	ClearCategory           bool       `json:"clear_category,omitempty"`
	ItemType                *string    `json:"item_type,omitempty"`
	Size                    *string    `json:"size,omitempty"`
	Unit                    *string    `json:"unit,omitempty"`
	Condition               *string    `json:"condition,omitempty"`
	LocationID              *uuid.UUID `json:"location_id,omitempty"`
	LocationName            string     `json:"location_name,omitempty"`
	ClearLocation           bool       `json:"clear_location,omitempty"`
	ParentContainerID       *uuid.UUID `json:"parent_container_id,omitempty"`
	ClearContainer          bool       `json:"clear_container,omitempty"`
	ParentGroupID           *uuid.UUID `json:"parent_group_id,omitempty"`
	ClearGroup              bool       `json:"clear_group,omitempty"`
	ManagedByDepartmentID   *uuid.UUID `json:"managed_by_department_id,omitempty"`
	ManagedByDepartmentName string     `json:"managed_by_department_name,omitempty"`
	ClearManagedBy          bool       `json:"clear_managed_by,omitempty"`
	NeededByDepartmentID    *uuid.UUID `json:"needed_by_department_id,omitempty"`
	NeededByDepartmentName  string     `json:"needed_by_department_name,omitempty"`
	ClearNeededBy           bool       `json:"clear_needed_by,omitempty"`
	SerialNumber            *string    `json:"serial_number,omitempty"`
	AssetTag                *string    `json:"asset_tag,omitempty"`
	QRCode                  *string    `json:"qr_code,omitempty"`
	Owner                   *string    `json:"owner,omitempty"`
	PurchasePrice           *string    `json:"purchase_price,omitempty"`
	ClearPurchasePrice      bool       `json:"clear_purchase_price,omitempty"`
	PurchaseCurrency        *string    `json:"purchase_currency,omitempty"`
	PurchaseDate            *string    `json:"purchase_date,omitempty"`
	ClearPurchaseDate       bool       `json:"clear_purchase_date,omitempty"`
	ExpiryDate              *string    `json:"expiry_date,omitempty"`
	ClearExpiryDate         bool       `json:"clear_expiry_date,omitempty"`
	Supplier                *string    `json:"supplier,omitempty"`
	PurchaseURL             *string    `json:"purchase_url,omitempty"`
	ReplacementCost         *string    `json:"replacement_cost,omitempty"`
	ClearReplacementCost    bool       `json:"clear_replacement_cost,omitempty"`
	MinimumQuantity         *int32     `json:"minimum_quantity,omitempty"`
	Notes                   *string    `json:"notes,omitempty"`
}

func (h *itemHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req itemUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	purchaseDate, err := parseDate(req.PurchaseDate)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	expiryDate, err := parseDate(req.ExpiryDate)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Update(r.Context(), id, service.UpdateItemInput{
		Name:                  req.Name,
		Description:           req.Description,
		CategoryID:            req.CategoryID,
		CategoryName:          req.CategoryName,
		ClearCategory:         req.ClearCategory,
		ItemType:              req.ItemType,
		Size:                  req.Size,
		Unit:                  req.Unit,
		Condition:             req.Condition,
		LocationID:            req.LocationID,
		LocationName:          req.LocationName,
		ClearLocation:         req.ClearLocation,
		ParentContainerID:     req.ParentContainerID,
		ClearContainer:        req.ClearContainer,
		ParentGroupID:         req.ParentGroupID,
		ClearGroup:            req.ClearGroup,
		ManagedByDepartmentID: req.ManagedByDepartmentID,
		ManagedByDeptName:     req.ManagedByDepartmentName,
		ClearManagedBy:        req.ClearManagedBy,
		NeededByDepartmentID:  req.NeededByDepartmentID,
		NeededByDeptName:      req.NeededByDepartmentName,
		ClearNeededBy:         req.ClearNeededBy,
		SerialNumber:          req.SerialNumber,
		AssetTag:              req.AssetTag,
		QRCode:                req.QRCode,
		Owner:                 req.Owner,
		PurchasePrice:         req.PurchasePrice,
		ClearPurchasePrice:    req.ClearPurchasePrice,
		PurchaseCurrency:      req.PurchaseCurrency,
		PurchaseDate:          purchaseDate,
		ClearPurchaseDate:     req.ClearPurchaseDate,
		ExpiryDate:            expiryDate,
		ClearExpiryDate:       req.ClearExpiryDate,
		Supplier:              req.Supplier,
		PurchaseURL:           req.PurchaseURL,
		ReplacementCost:       req.ReplacementCost,
		ClearReplacementCost:  req.ClearReplacementCost,
		MinimumQuantity:       req.MinimumQuantity,
		Notes:                 req.Notes,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

func (h *itemHandler) availability(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	a, err := h.svc.Availability(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// ----------------------------------------------------------------------------
// Lifecycle endpoints
// ----------------------------------------------------------------------------

type moveRequest struct {
	ToLocationID   *uuid.UUID `json:"to_location_id,omitempty"`
	ClearLocation  bool       `json:"clear_location,omitempty"`
	ToContainerID  *uuid.UUID `json:"to_container_id,omitempty"`
	ClearContainer bool       `json:"clear_container,omitempty"`
	Note           string     `json:"note,omitempty"`
}

func (h *itemHandler) move(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req moveRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Move(r.Context(), id, service.MoveInput{
		ToLocationID:   req.ToLocationID,
		ClearLocation:  req.ClearLocation,
		ToContainerID:  req.ToContainerID,
		ClearContainer: req.ClearContainer,
		Note:           req.Note,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

type adjustRequest struct {
	Delta int32  `json:"delta"`
	Note  string `json:"note,omitempty"`
}

func (h *itemHandler) adjust(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req adjustRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.AdjustQuantity(r.Context(), id, service.AdjustInput{
		Delta: req.Delta,
		Note:  req.Note,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

type quantityNoteRequest struct {
	Quantity int32  `json:"quantity"`
	Note     string `json:"note,omitempty"`
}

type repairRequest struct {
	Quantity       int32      `json:"quantity"`
	Note           string     `json:"note,omitempty"`
	RelatedEventID *uuid.UUID `json:"related_event_id,omitempty"`
}

func (h *itemHandler) damage(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req quantityNoteRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.MarkDamaged(r.Context(), id, service.DamageInput{Quantity: req.Quantity, Note: req.Note})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

func (h *itemHandler) repair(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req repairRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Repair(r.Context(), id, service.RepairInput{
		Quantity:       req.Quantity,
		Note:           req.Note,
		RelatedEventID: req.RelatedEventID,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

func (h *itemHandler) lost(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req quantityNoteRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.MarkLost(r.Context(), id, service.LostInput{Quantity: req.Quantity, Note: req.Note})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

func (h *itemHandler) found(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req quantityNoteRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.MarkFound(r.Context(), id, service.LostInput{Quantity: req.Quantity, Note: req.Note})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

type noteRequest struct {
	Note string `json:"note,omitempty"`
}

func (h *itemHandler) retire(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req noteRequest
	_ = decodeJSON(r, &req)
	row, err := h.svc.Retire(r.Context(), id, req.Note)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

func (h *itemHandler) restore(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req noteRequest
	_ = decodeJSON(r, &req)
	row, err := h.svc.Restore(r.Context(), id, req.Note)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(row))
}

// ----------------------------------------------------------------------------
// History
// ----------------------------------------------------------------------------

type itemEventDTO struct {
	ID               string  `json:"id"`
	ItemID           string  `json:"item_id"`
	EventType        string  `json:"event_type"`
	Quantity         int32   `json:"quantity"`
	FromLocationID   *string `json:"from_location_id,omitempty"`
	ToLocationID     *string `json:"to_location_id,omitempty"`
	FromDepartmentID *string `json:"from_department_id,omitempty"`
	ToDepartmentID   *string `json:"to_department_id,omitempty"`
	RelatedSignoutID *string `json:"related_signout_id,omitempty"`
	RelatedEventID   *string `json:"related_event_id,omitempty"`
	RelatedFileID    *string `json:"related_file_id,omitempty"`
	ConditionBefore  *string `json:"condition_before,omitempty"`
	ConditionAfter   *string `json:"condition_after,omitempty"`
	ActorUserID      *string `json:"actor_user_id,omitempty"`
	ActorOIDCSubject string  `json:"actor_oidc_subject,omitempty"`
	ActorDisplayName string  `json:"actor_display_name,omitempty"`
	Note             string  `json:"note"`
	OccurredAt       string  `json:"occurred_at"`
}

func toItemEventDTO(e sqlcgen.ItemEvent) itemEventDTO {
	return itemEventDTO{
		ID:               e.ID.String(),
		ItemID:           e.ItemID.String(),
		EventType:        e.EventType,
		Quantity:         e.Quantity,
		FromLocationID:   uuidToStrPtr(e.FromLocationID),
		ToLocationID:     uuidToStrPtr(e.ToLocationID),
		FromDepartmentID: uuidToStrPtr(e.FromDepartmentID),
		ToDepartmentID:   uuidToStrPtr(e.ToDepartmentID),
		RelatedSignoutID: uuidToStrPtr(e.RelatedSignoutID),
		RelatedEventID:   uuidToStrPtr(e.RelatedEventID),
		RelatedFileID:    uuidToStrPtr(e.RelatedFileID),
		ConditionBefore:  e.ConditionBefore,
		ConditionAfter:   e.ConditionAfter,
		ActorUserID:      uuidToStrPtr(e.ActorUserID),
		ActorOIDCSubject: strOrEmpty(e.ActorOidcSubject),
		ActorDisplayName: strOrEmpty(e.ActorDisplayName),
		Note:             e.Note,
		OccurredAt:       timestampString(e.OccurredAt),
	}
}

func (h *itemHandler) history(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	signoutID, err := queryUUID(r, "signout_id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	since, err := queryTime(r, "since")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	res, err := h.svc.ListHistory(r.Context(), id, service.ListHistoryInput{
		EventType: queryString(r, "event_type"),
		SignoutID: signoutID,
		Since:     since,
		Limit:     queryInt(r, "limit", 100),
		Offset:    queryInt(r, "offset", 0),
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out := make([]itemEventDTO, 0, len(res.Events))
	for _, e := range res.Events {
		out = append(out, toItemEventDTO(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": out, "total": res.Total})
}
