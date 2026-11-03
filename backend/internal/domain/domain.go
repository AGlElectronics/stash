package domain

import "strings"

type ItemType string

const (
	ItemTypeAsset      ItemType = "asset"
	ItemTypeConsumable ItemType = "consumable"
	ItemTypeContainer  ItemType = "container"
	ItemTypeGrouped    ItemType = "grouped"
)

func (t ItemType) Valid() bool {
	switch t {
	case ItemTypeAsset, ItemTypeConsumable, ItemTypeContainer, ItemTypeGrouped:
		return true
	}
	return false
}

type ItemStatus string

const (
	ItemStatusAvailable ItemStatus = "available"
	ItemStatusInUse     ItemStatus = "in_use"
	ItemStatusRetired   ItemStatus = "retired"
)

type SignoutStatus string

const (
	SignoutStatusDraft             SignoutStatus = "draft"
	SignoutStatusPrepared          SignoutStatus = "prepared"
	SignoutStatusSignedOut         SignoutStatus = "signed_out"
	SignoutStatusPartiallyReturned SignoutStatus = "partially_returned"
	SignoutStatusReturned          SignoutStatus = "returned"
	SignoutStatusClosed            SignoutStatus = "closed"
	SignoutStatusCancelled         SignoutStatus = "cancelled"
)

type ItemEventType string

const (
	EventCreated          ItemEventType = "created"
	EventMoved            ItemEventType = "moved"
	EventQuantityAdjusted ItemEventType = "quantity_adjusted"
	EventSignedOut        ItemEventType = "signed_out"
	EventReturned         ItemEventType = "returned"
	EventConsumed         ItemEventType = "consumed"
	EventMarkedDamaged    ItemEventType = "marked_damaged"
	EventRepaired         ItemEventType = "repaired"
	EventMarkedLost       ItemEventType = "marked_lost"
	EventMarkedFound      ItemEventType = "marked_found"
	EventConditionChanged ItemEventType = "condition_changed"
	EventRetired          ItemEventType = "retired"
	EventRestored         ItemEventType = "restored"
)

// FileType enumerates the kinds of files we attach to an entity.
type FileType string

const (
	FileTypeImage      FileType = "image"
	FileTypeReceipt    FileType = "receipt"
	FileTypeInvoice    FileType = "invoice"
	FileTypeManual     FileType = "manual"
	FileTypeWarranty   FileType = "warranty"
	FileTypeAttachment FileType = "attachment"
)

func (t FileType) Valid() bool {
	switch t {
	case FileTypeImage, FileTypeReceipt, FileTypeInvoice, FileTypeManual, FileTypeWarranty, FileTypeAttachment:
		return true
	}
	return false
}

// LostFoundType says whether a guest lost something or something was found.
type LostFoundType string

const (
	LostFoundTypeLost  LostFoundType = "lost"
	LostFoundTypeFound LostFoundType = "found"
)

func (t LostFoundType) Valid() bool {
	switch t {
	case LostFoundTypeLost, LostFoundTypeFound:
		return true
	}
	return false
}

// LostFoundStatus is the lifecycle of a lost & found report. "returned" means
// the item was handed back to its owner; "closed" means unclaimed / disposed /
// donated at end of con. Both can be reopened back to "open".
type LostFoundStatus string

const (
	LostFoundStatusOpen     LostFoundStatus = "open"
	LostFoundStatusReturned LostFoundStatus = "returned"
	LostFoundStatusClosed   LostFoundStatus = "closed"
)

func (s LostFoundStatus) Valid() bool {
	switch s {
	case LostFoundStatusOpen, LostFoundStatusReturned, LostFoundStatusClosed:
		return true
	}
	return false
}

// SacredLostFoundReportNumber is the immortal first report — the funny test item.
const SacredLostFoundReportNumber = "LF-0001"

func LostFoundReportIsSacred(reportNumber string) bool {
	return strings.EqualFold(strings.TrimSpace(reportNumber), SacredLostFoundReportNumber)
}

// Permission names. This in-code catalog is the single source of truth; the
// access config file (roles section) may only reference permissions listed
// here.
const (
	PermItemView         = "item.view"
	PermItemCreate       = "item.create"
	PermItemEdit         = "item.edit"
	PermItemRetire       = "item.retire"
	PermItemMove         = "item.move"
	PermItemAdjust       = "item.adjust"
	PermItemDamage       = "item.damage"
	PermItemLose         = "item.lose"
	PermSignoutView      = "signout.view"
	PermSignoutCreate    = "signout.create"
	PermSignoutConfirm   = "signout.confirm"
	PermSignoutReturn    = "signout.return"
	PermSignoutCancel    = "signout.cancel"
	PermSignoutClose     = "signout.close"
	PermDepartmentManage = "department.manage"
	PermLocationManage   = "location.manage"
	PermCategoryManage   = "category.manage"
	PermUserManage       = "user.manage"
	PermAccessManage     = "access.manage"
	PermAuditView        = "audit.view"
	PermExportCreate     = "export.create"
	PermFileUpload       = "file.upload"
	PermLostfoundView    = "lostfound.view"
	PermLostfoundCreate  = "lostfound.create"
	PermLostfoundEdit    = "lostfound.edit"
	PermLostfoundResolve = "lostfound.resolve"
	PermLostfoundReopen  = "lostfound.reopen"
	PermLostfoundDelete  = "lostfound.delete"
)

// AllPermissions returns every permission in the catalog. Used for the
// admin role default and for validating the access config file.
func AllPermissions() []string {
	return []string{
		PermItemView, PermItemCreate, PermItemEdit, PermItemRetire,
		PermItemMove, PermItemAdjust, PermItemDamage, PermItemLose,
		PermSignoutView, PermSignoutCreate, PermSignoutConfirm,
		PermSignoutReturn, PermSignoutCancel, PermSignoutClose,
		PermDepartmentManage, PermLocationManage, PermCategoryManage,
		PermUserManage, PermAccessManage, PermAuditView,
		PermExportCreate, PermFileUpload,
		PermLostfoundView, PermLostfoundCreate, PermLostfoundEdit, PermLostfoundResolve,
		PermLostfoundReopen, PermLostfoundDelete,
	}
}

// Role names of the built-in roles. The access config file may define
// additional roles or override the permission set of these.
const (
	RoleAdmin            = "admin"
	RoleInventoryManager = "inventory_manager"
	RoleDeptLead         = "dept_lead"
	RoleVolunteer        = "volunteer"
	RoleReadonly         = "readonly"
	RoleFrontdesk        = "frontdesk"
	RoleConopsLead       = "conops_lead"
)

// DefaultRolePermissions is the built-in role → permission catalog, used when
// the access config file does not (re)define a role. Lost & found permissions
// are deliberately granted only to admin and frontdesk: the whole section is
// invisible to everyone else (user decision from the frontend redesign).
func DefaultRolePermissions() map[string][]string {
	return map[string][]string{
		RoleAdmin: AllPermissions(),
		RoleInventoryManager: {
			PermItemView, PermItemCreate, PermItemEdit, PermItemRetire,
			PermItemMove, PermItemAdjust, PermItemDamage, PermItemLose,
			PermSignoutView, PermSignoutCreate, PermSignoutConfirm,
			PermSignoutReturn, PermSignoutCancel, PermSignoutClose,
			PermDepartmentManage, PermLocationManage, PermCategoryManage,
			PermAuditView, PermExportCreate, PermFileUpload,
		},
		RoleDeptLead: {
			PermItemView, PermItemDamage, PermItemLose,
			PermSignoutView, PermSignoutCreate, PermSignoutConfirm,
			PermSignoutReturn, PermSignoutCancel, PermSignoutClose,
			PermFileUpload, PermAuditView, PermExportCreate,
		},
		RoleVolunteer: {
			PermItemView, PermSignoutView, PermSignoutCreate, PermSignoutReturn,
		},
		RoleReadonly: {
			PermItemView, PermSignoutView, PermAuditView,
		},
		RoleFrontdesk: {
			PermLostfoundView, PermLostfoundCreate, PermLostfoundResolve,
			PermFileUpload,
		},
		RoleConopsLead: {
			PermLostfoundView, PermLostfoundCreate, PermLostfoundEdit, PermLostfoundResolve,
			PermLostfoundReopen, PermLostfoundDelete,
			PermAuditView, PermExportCreate, PermFileUpload,
		},
	}
}
