package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

type signoutHandler struct {
	svc *service.SignoutService
}

type signoutDTO struct {
	ID                string  `json:"id"`
	SignoutNumber     string  `json:"signout_number"`
	FromDepartmentID  *string `json:"from_department_id,omitempty"`
	ToDepartmentID    string  `json:"to_department_id"`
	FromLocationID    *string `json:"from_location_id,omitempty"`
	Status            string  `json:"status"`
	Purpose           string  `json:"purpose"`
	SignedOutByUserID *string `json:"signed_out_by_user_id,omitempty"`
	ReceivedByUserID  *string `json:"received_by_user_id,omitempty"`
	ReceivedByName    *string `json:"received_by_name,omitempty"`
	ExpectedReturnAt  string  `json:"expected_return_at,omitempty"`
	SignedOutAt       string  `json:"signed_out_at,omitempty"`
	ReturnedAt        string  `json:"returned_at,omitempty"`
	ClosedAt          string  `json:"closed_at,omitempty"`
	CancelledAt       string  `json:"cancelled_at,omitempty"`
	Notes             string  `json:"notes"`
	CreatedBy         *string `json:"created_by,omitempty"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

type signoutItemDTO struct {
	ID                 string  `json:"id"`
	SignoutID          string  `json:"signout_id"`
	ItemID             string  `json:"item_id"`
	QuantitySignedOut  int32   `json:"quantity_signed_out"`
	QuantityReturned   int32   `json:"quantity_returned"`
	QuantityConsumed   int32   `json:"quantity_consumed"`
	QuantityLost       int32   `json:"quantity_lost"`
	QuantityDamaged    int32   `json:"quantity_damaged"`
	ConditionOnSignout *string `json:"condition_on_signout,omitempty"`
	ConditionOnReturn  *string `json:"condition_on_return,omitempty"`
	Notes              string  `json:"notes"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type signoutWithItemsDTO struct {
	signoutDTO
	Items []signoutItemDTO `json:"items"`
}

func toSignoutDTO(s sqlcgen.DepartmentSignout) signoutDTO {
	return signoutDTO{
		ID:                uuidPtrToString(&s.ID),
		SignoutNumber:     s.SignoutNumber,
		FromDepartmentID:  uuidPtrToString2(s.FromDepartmentID),
		ToDepartmentID:    s.ToDepartmentID.String(),
		FromLocationID:    uuidPtrToString2(s.FromLocationID),
		Status:            s.Status,
		Purpose:           s.Purpose,
		SignedOutByUserID: uuidPtrToString2(s.SignedOutByUserID),
		ReceivedByUserID:  uuidPtrToString2(s.ReceivedByUserID),
		ReceivedByName:    s.ReceivedByName,
		ExpectedReturnAt:  timestampString(s.ExpectedReturnAt),
		SignedOutAt:       timestampString(s.SignedOutAt),
		ReturnedAt:        timestampString(s.ReturnedAt),
		ClosedAt:          timestampString(s.ClosedAt),
		CancelledAt:       timestampString(s.CancelledAt),
		Notes:             s.Notes,
		CreatedBy:         uuidPtrToString2(s.CreatedBy),
		CreatedAt:         timestampString(s.CreatedAt),
		UpdatedAt:         timestampString(s.UpdatedAt),
	}
}

func toSignoutItemDTO(s sqlcgen.DepartmentSignoutItem) signoutItemDTO {
	return signoutItemDTO{
		ID:                 s.ID.String(),
		SignoutID:          s.SignoutID.String(),
		ItemID:             s.ItemID.String(),
		QuantitySignedOut:  s.QuantitySignedOut,
		QuantityReturned:   s.QuantityReturned,
		QuantityConsumed:   s.QuantityConsumed,
		QuantityLost:       s.QuantityLost,
		QuantityDamaged:    s.QuantityDamaged,
		ConditionOnSignout: s.ConditionOnSignout,
		ConditionOnReturn:  s.ConditionOnReturn,
		Notes:              s.Notes,
		CreatedAt:          timestampString(s.CreatedAt),
		UpdatedAt:          timestampString(s.UpdatedAt),
	}
}

func toSignoutWithItemsDTO(s service.SignoutWithItems) signoutWithItemsDTO {
	items := make([]signoutItemDTO, 0, len(s.Items))
	for _, it := range s.Items {
		items = append(items, toSignoutItemDTO(it))
	}
	return signoutWithItemsDTO{
		signoutDTO: toSignoutDTO(s.Signout),
		Items:      items,
	}
}

func uuidPtrToString(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}

func uuidPtrToString2(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

func (h *signoutHandler) routes(r chi.Router) {
	r.With(requireAny(domain.PermSignoutView)).Get("/", h.list)
	r.With(requireAny(domain.PermSignoutCreate)).Post("/", h.create)
	r.With(requireAny(domain.PermSignoutView)).Get("/{id}", h.get)
	r.With(requireAny(domain.PermSignoutConfirm)).Post("/{id}/confirm", h.confirm)
	r.With(requireAny(domain.PermSignoutReturn)).Post("/{id}/return", h.returnLines)
	r.With(requireAny(domain.PermSignoutCancel)).Post("/{id}/cancel", h.cancel)
	r.With(requireAny(domain.PermSignoutClose)).Post("/{id}/close", h.close)
}

func (h *signoutHandler) list(w http.ResponseWriter, r *http.Request) {
	in := service.ListSignoutsInput{
		Search: r.URL.Query().Get("q"),
		Status: r.URL.Query().Get("status"),
		Limit:  queryInt(r, "limit", 50),
		Offset: queryInt(r, "offset", 0),
	}
	if v := r.URL.Query().Get("to_department_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid to_department_id", nil))
			return
		}
		in.ToDeptID = &id
	}
	if v := r.URL.Query().Get("from_department_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid from_department_id", nil))
			return
		}
		in.FromDeptID = &id
	}

	res, err := h.svc.List(r.Context(), in)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out := make([]signoutDTO, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, toSignoutDTO(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out,
		"total": res.Total,
	})
}

func (h *signoutHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSignoutWithItemsDTO(out))
}

type createSignoutItemReq struct {
	ItemID             string   `json:"item_id"`
	Quantity           int32    `json:"quantity"`
	ConditionOnSignout *string  `json:"condition_on_signout,omitempty"`
	Notes              string   `json:"notes"`
	ScannedCodes       []string `json:"scanned_codes,omitempty"`
}

type createSignoutReq struct {
	FromDepartmentID *string                `json:"from_department_id,omitempty"`
	ToDepartmentID   string                 `json:"to_department_id"`
	FromLocationID   *string                `json:"from_location_id,omitempty"`
	Purpose          string                 `json:"purpose"`
	ExpectedReturnAt *string                `json:"expected_return_at,omitempty"`
	ReceivedByUserID *string                `json:"received_by_user_id,omitempty"`
	ReceivedByName   string                 `json:"received_by_name"`
	Notes            string                 `json:"notes"`
	Items            []createSignoutItemReq `json:"items"`
	Confirm          bool                   `json:"confirm"`
}

func (h *signoutHandler) create(w http.ResponseWriter, r *http.Request) {
	var body createSignoutReq
	if err := decodeJSON(r, &body); err != nil {
		httperror.Render(w, r, err)
		return
	}
	toDept, err := uuid.Parse(body.ToDepartmentID)
	if err != nil {
		httperror.Render(w, r, httperror.Validation("invalid to_department_id", nil))
		return
	}
	in := service.CreateSignoutInput{
		ToDepartmentID: toDept,
		Purpose:        body.Purpose,
		ReceivedByName: body.ReceivedByName,
		Notes:          body.Notes,
		Confirm:        body.Confirm,
	}
	if body.FromDepartmentID != nil && *body.FromDepartmentID != "" {
		id, err := uuid.Parse(*body.FromDepartmentID)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid from_department_id", nil))
			return
		}
		in.FromDepartmentID = &id
	}
	if body.FromLocationID != nil && *body.FromLocationID != "" {
		id, err := uuid.Parse(*body.FromLocationID)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid from_location_id", nil))
			return
		}
		in.FromLocationID = &id
	}
	if body.ReceivedByUserID != nil && *body.ReceivedByUserID != "" {
		id, err := uuid.Parse(*body.ReceivedByUserID)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid received_by_user_id", nil))
			return
		}
		in.ReceivedByUserID = &id
	}
	if body.ExpectedReturnAt != nil && *body.ExpectedReturnAt != "" {
		t, err := time.Parse(time.RFC3339, *body.ExpectedReturnAt)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid expected_return_at (use RFC3339)", nil))
			return
		}
		in.ExpectedReturnAt = &t
	}
	for i, line := range body.Items {
		itemID, err := uuid.Parse(line.ItemID)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid item_id at items["+strconv.Itoa(i)+"]", nil))
			return
		}
		in.Items = append(in.Items, service.SignoutItemInput{
			ItemID:             itemID,
			Quantity:           line.Quantity,
			ConditionOnSignout: line.ConditionOnSignout,
			Notes:              line.Notes,
			ScannedCodes:       line.ScannedCodes,
		})
	}

	out, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toSignoutWithItemsDTO(out))
}

func (h *signoutHandler) confirm(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Confirm(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSignoutWithItemsDTO(out))
}

type returnLineReq struct {
	SignoutItemID     string  `json:"signout_item_id"`
	Returned          int32   `json:"returned"`
	Consumed          int32   `json:"consumed"`
	Lost              int32   `json:"lost"`
	Damaged           int32   `json:"damaged"`
	ConditionOnReturn *string `json:"condition_on_return,omitempty"`
}

type returnReq struct {
	Lines []returnLineReq `json:"lines"`
}

func (h *signoutHandler) returnLines(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var body returnReq
	if err := decodeJSON(r, &body); err != nil {
		httperror.Render(w, r, err)
		return
	}
	lines := make([]service.ReturnLine, 0, len(body.Lines))
	for i, l := range body.Lines {
		lid, err := uuid.Parse(l.SignoutItemID)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid signout_item_id at lines["+strconv.Itoa(i)+"]", nil))
			return
		}
		lines = append(lines, service.ReturnLine{
			SignoutItemID:     lid,
			ReturnedDelta:     l.Returned,
			ConsumedDelta:     l.Consumed,
			LostDelta:         l.Lost,
			DamagedDelta:      l.Damaged,
			ConditionOnReturn: l.ConditionOnReturn,
		})
	}
	out, err := h.svc.Return(r.Context(), id, lines)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSignoutWithItemsDTO(out))
}

type cancelReq struct {
	Note string `json:"note"`
}

func (h *signoutHandler) cancel(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var body cancelReq
	_ = decodeJSON(r, &body)
	out, err := h.svc.Cancel(r.Context(), id, body.Note)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSignoutWithItemsDTO(out))
}

func (h *signoutHandler) close(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Close(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSignoutWithItemsDTO(out))
}
