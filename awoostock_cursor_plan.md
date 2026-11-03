# Stash, Inventory Management System for Awoostria

## 1. Project overview

Stash is an internal inventory management system for Awoostria. It is designed to manage convention equipment, consumables, containers, storage locations, department handovers, item images, purchase information, event packing, and returns.

The system is not just a simple list of items. It must track responsibility and movement, especially during event preparation and the convention itself.

The core workflow is:

```text
Inventory storage -> prepared for department -> signed out to department -> used / returned / consumed / damaged / lost
```

The system should answer questions like:

```text
What do we own?
How many of each item do we have?
Where is an item currently located?
Which department has it?
Who signed it out?
When should it come back?
What was bought, when, and for what price?
What items are missing, damaged, low stock, or still signed out?
What does each department need for the convention?
```

## 2. Recommended project name

Primary name:

```text
Stash
```

Recommended full product name:

```text
Awoostria Stash
```

Recommended repository name:

```text
stash
```

Reasoning:

```text
Short and memorable
Easy to type in a browser search bar or app launcher
Fits the furry convention vibe without being too forced or too jokey
Naturally describes stored equipment, supplies, consumables, and event gear
Works well in normal speech, for example: check Stash, add it to Stash, sign it out in Stash
Broad enough to cover inventory, purchasing, sign-outs, returns, damaged items, and department responsibility
More flexible than names focused only on crates, boxes, stock, or monitoring
Good fit for a Go based internal operations tool
```

Recommended tagline:

```text
Stash, inventory, sign-out, and department handover management for Awoostria.
```

Name direction:

```text
Use Stash as the actual project name.
Use stash as the repository and binary name.
Use Awoostria Stash as the UI title where extra branding is useful.
Use Awoostria Inventory Management System as the descriptive subtitle.
Avoid AwooStock, CrateWatch, and GearWatch as final names. They are understandable, but Stash is shorter, friendlier, easier to remember, and better aligned with the convention context.
```

Alternative names considered:

```text
CrateWatch, close, but too focused on crates and boxes
GearWatch, good general name, but still feels like another monitoring tool
StockWatch, clear, but too generic
KitWatch, good for event kits, weaker for general inventory and consumables
Hoard, memorable and thematic, but can imply messy storage
Fetch, good for requests, but less clear for inventory ownership
Burrow, thematic, but too broad and less operational
AwooVault, nice branding, but sounds more like secure storage
AwooDepot, decent, but weaker than Stash
Conventory, clever, but less serious and less clean
```

## 2.1 Project style reference

Stash should follow the same general engineering style as the Vinuja project, because that project structure and implementation style worked well.

The goal is not to copy code blindly, but to keep a similar feeling:

```text
Go backend
Clean package structure
Small focused services
Simple readable code
Explicit configuration
Clear separation between handlers, services, repositories, database queries, and domain logic
PostgreSQL first
Docker based local development
Practical production-minded structure from the start
```

Cursor should treat Vinuja as the preferred style reference when making architectural choices, naming packages, organizing backend code, and keeping the Go implementation clean and maintainable.

## 3. High level goals

Stash should support:

```text
Item inventory
Quantity tracking
Purchase price tracking
Department based responsibility
Department sign-out and return workflow
Consumable usage tracking
Reusable asset return tracking
Location and container tracking
Item pictures and receipt uploads
Audit logging
CSV import and export
Future QR code scanning
Future event packing lists
Future OIDC authentication
```

## 4. Core domain concepts

### 4.1 Items

An item is anything Awoostria owns, stores, lends, consumes, packs, or tracks.

Examples:

```text
Badge printers
Barcode scanners
Laptops
Network switches
Access points
Power strips
Extension cables
Radios
First aid kits
Tape rolls
Cable ties
Badge holders
Lanyards
Decoration material
Toolboxes
Signage
```

Items can be one of several types:

```text
asset
consumable
container
```

#### Asset

A reusable item that is expected to return.

Examples:

```text
Laptop
Printer
Radio
Switch
Barcode scanner
Toolbox
```

#### Consumable

An item that can be used up.

Examples:

```text
Tape
Badge holders
Lanyards
Cable ties
Printer paper
Batteries
```

#### Container

A box, crate, flight case, bag, or kit that can contain other items.

Examples:

```text
Registration box
Network crate
AV kit
First aid bag
Decoration crate
```

### 4.2 Departments

Departments are first-class entities. They are used for ownership, planning, and sign-outs.

Example departments:

```text
Registration
ConOps
IT / Infrastructure
Security
First Aid
Dealers Den
Main Stage
AV / Tech
Decoration
Logistics
Volunteers
Board
Hotel / Venue
```

Items should distinguish between:

```text
Managed by department
Needed by department
Currently signed out to department
```

These are not always the same.

Example:

```text
Item: Barcode scanner
Managed by: IT / Infrastructure
Needed by: Registration
Currently signed out to: Registration
```

### 4.3 Locations

Locations represent where items physically are.

Locations should be hierarchical.

Examples:

```text
Main storage
Main storage / Shelf A
Main storage / Shelf A / Box 3
Venue
Venue / Registration desk
Venue / ConOps room
Venue / Tech room
Borrowed / External
Disposed
```

### 4.4 Containers

Containers are movable storage units that can contain items.

A container should support:

```text
Name
Description
Location
Status
QR code later
List of contained items
Movement history
```

Moving a container should effectively move its contents with it.

### 4.5 Sign-outs

A sign-out is a temporary handover of items from storage or one department to another department.

It represents responsibility, not permanent ownership.

Example:

```text
Sign-out: SO-2026-00012
From: Logistics
To: Registration
Purpose: Registration desk setup
Expected return: End of convention
Signed out by: Logistics volunteer
Received by: Registration lead
Items:
  4x barcode scanners
  2x badge printers
  20x power strips
  1500x badge holders
```

The sign-out system must track:

```text
What was signed out
How many were signed out
To which department
Who accepted the items
Expected return date
Returned quantity
Consumed quantity
Damaged quantity
Lost quantity
Condition on sign-out
Condition on return
Notes
```

## 5. Primary features

### 5.1 Inventory management

The system must support:

```text
Create item
Edit item
Retire item
View item details
Search items
Filter items by category
Filter items by department
Filter items by location
Filter items by condition
Filter items by status
Track item quantity
Track item unit
Track item condition
Track item status
Track item purchase information
Upload item image
Upload receipt or invoice
View item movement history
View current availability
```

### 5.2 Item fields

Each item should have:

```text
Item name
Description
Category
Item type
Quantity owned
Quantity available
Unit
Condition
Status
Storage location
Container
Managed by department
Needed by department
Serial number, optional
Internal asset tag, optional
QR code, later
Purchase price
Purchase currency
Purchase date
Supplier / vendor
Purchase URL, optional
Receipt / invoice attachment
Primary item image
Replacement cost
Minimum stock quantity
Owner, Awoostria / private / rented / borrowed
Notes
Tags
Created by
Created at
Updated at
Retired at
```

### 5.3 Department sign-out system

The system must support creating and managing department sign-outs.

Sign-out statuses:

```text
Draft
Prepared
Signed out
Partially returned
Returned
Closed
Cancelled
```

Line item result states:

```text
Returned
Consumed
Damaged
Lost
Still outstanding
```

Required sign-out features:

```text
Create sign-out
Add items and quantities
Check item availability
Assign destination department
Assign source department or source location
Set expected return date
Set purpose
Confirm sign-out
Record who signed it out
Record who received it
Return items partially or fully
Mark consumables as consumed
Mark assets as damaged or lost
Close sign-out
Show all active sign-outs
Show all sign-outs for a department
Show all sign-outs for an item
```

### 5.4 Purchase tracking

For MVP, purchase fields can exist directly on items.

The system should track:

```text
Purchase price
Currency
Purchase date
Supplier
Purchase URL
Receipt or invoice file
Replacement cost
Department purchase was for
```

Later, this can be expanded into proper purchase records and purchase requests.

### 5.5 Item requests, future feature

Departments should eventually be able to request items.

Example request:

```text
Department: Registration
Requested item: Badge holders
Quantity needed: 1500
Needed by date: 2026-05-15
Priority: High
Estimated price: EUR 120
Reason: Needed for attendee check-in
Status: Requested
```

Possible request statuses:

```text
Draft
Requested
Approved
Rejected
Ordered
Partially received
Received
Cancelled
```

This is not required for the first MVP, but the data model should not block it.

### 5.6 Item images and file attachments

The system must support item pictures.

Files should not be stored inside PostgreSQL. Use S3-compatible object storage.

Recommended storage:

```text
Local development: MinIO
Production: S3-compatible storage, for example MinIO, AWS S3, Cloudflare R2, Hetzner Object Storage, or another S3-compatible provider
```

File types:

```text
Item image
Receipt
Invoice
Manual
Warranty document
Other attachment
```

Each file should store metadata in PostgreSQL and the actual object in S3-compatible storage.

Metadata fields:

```text
id
entity type
entity id
storage bucket
storage key
original filename
content type
size bytes
uploaded by
created at
```

### 5.7 Audit log

Every important state change should be logged.

Examples:

```text
Item created
Item edited
Quantity adjusted
Item moved
Item signed out
Item returned
Item marked damaged
Item marked lost
Image uploaded
Receipt uploaded
Purchase price changed
Department changed
Location changed
```

The audit log should store:

```text
Actor
Action
Entity type
Entity ID
Before value
After value
Timestamp
Optional note
```

### 5.8 Import and export

MVP should support CSV export.

Useful exports:

```text
All items
Items by department
Items by location
Low stock items
Active sign-outs
Damaged items
Missing items
Purchase overview
```

Later, support CSV import and bulk update.

## 6. Tech stack

### 6.1 Backend

Use Go for the backend.

Recommended backend stack:

```text
Go
Chi HTTP router
PostgreSQL
pgx PostgreSQL driver
sqlc for typed SQL queries
Goose for database migrations
Zerolog or Zap for structured logging
S3-compatible object storage client
OIDC middleware later
```

Preferred choices:

```text
Router: Chi
Database: PostgreSQL
DB access: pgx + sqlc
Migrations: Goose
Config: environment variables, possibly Viper later
Logging: Zerolog
File storage: S3-compatible API
```

### 6.2 Frontend

Use React with TypeScript.

Recommended frontend stack:

```text
React
TypeScript
Vite
TanStack Router
TanStack Query
React Hook Form
Zod
Tailwind CSS
shadcn/ui
Lucide icons
```

Future additions:

```text
PWA support
Camera based QR scanning
Offline mode for event use
```

### 6.3 Database

Use PostgreSQL as the main database.

PostgreSQL stores:

```text
Items
Departments
Categories
Locations
Containers
Quantities
Sign-outs
Purchase metadata
File metadata
Audit logs
Users
Roles
```

PostgreSQL should not store large binary files.

### 6.4 File storage

Use S3-compatible object storage.

Object storage stores:

```text
Item images
Receipts
Invoices
Manuals
Warranty files
Other attachments
```

Local development should use MinIO through Docker Compose.

Production can use any S3-compatible provider.

### 6.5 Authentication and authorization

For MVP, add a simple placeholder auth structure.

The system should be designed for future OIDC integration.

Future OIDC providers could be:

```text
Authentik
Keycloak
Zitadel
Google Workspace through an IdP
```

Roles:

```text
Admin
Inventory manager
Department lead
Volunteer
Read only
```

Permissions should eventually control:

```text
View inventory
Create items
Edit items
Retire items
Move items
Create sign-outs
Confirm sign-outs
Return items
Manage departments
Manage locations
Manage users
View audit log
Export data
```

## 7. Suggested monorepo layout

Repository name should be:

```text
stash
```

Suggested layout:

```text
stash/
  backend/
    cmd/
      server/
        main.go
    internal/
      audit/
      auth/
      config/
      db/
      files/
      http/
      repository/
      service/
    migrations/
    queries/
    sqlc.yaml
    go.mod
    go.sum

  frontend/
    src/
      api/
      components/
      forms/
      hooks/
      pages/
      routes/
      types/
      utils/
    package.json
    vite.config.ts
    tsconfig.json

  infra/
    docker-compose.yml
    Dockerfile.backend
    Dockerfile.frontend
    minio/
    postgres/

  README.md
```

## 8. Initial database model

This is a first-pass schema. It can be refined during implementation.

```sql
CREATE TABLE departments (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE item_categories (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE locations (
  id UUID PRIMARY KEY,
  parent_id UUID REFERENCES locations(id),
  name TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE items (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  category_id UUID REFERENCES item_categories(id),
  item_type TEXT NOT NULL,
  quantity_owned NUMERIC(12, 2) NOT NULL DEFAULT 0,
  unit TEXT NOT NULL DEFAULT 'pcs',
  condition TEXT NOT NULL DEFAULT 'good',
  status TEXT NOT NULL DEFAULT 'available',
  location_id UUID REFERENCES locations(id),
  container_id UUID REFERENCES items(id),
  managed_by_department_id UUID REFERENCES departments(id),
  needed_by_department_id UUID REFERENCES departments(id),
  serial_number TEXT,
  asset_tag TEXT,
  qr_code TEXT,
  owner TEXT NOT NULL DEFAULT 'awoostria',
  purchase_price NUMERIC(12, 2),
  purchase_currency TEXT DEFAULT 'EUR',
  purchase_date DATE,
  supplier TEXT,
  purchase_url TEXT,
  replacement_cost NUMERIC(12, 2),
  minimum_quantity NUMERIC(12, 2),
  primary_image_file_id UUID,
  notes TEXT,
  created_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  retired_at TIMESTAMPTZ
);

CREATE TABLE files (
  id UUID PRIMARY KEY,
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  file_type TEXT NOT NULL,
  bucket TEXT NOT NULL,
  storage_key TEXT NOT NULL,
  original_filename TEXT NOT NULL,
  content_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  uploaded_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE department_signouts (
  id UUID PRIMARY KEY,
  signout_number TEXT NOT NULL UNIQUE,
  from_department_id UUID REFERENCES departments(id),
  to_department_id UUID NOT NULL REFERENCES departments(id),
  from_location_id UUID REFERENCES locations(id),
  event_id UUID,
  status TEXT NOT NULL DEFAULT 'draft',
  purpose TEXT,
  signed_out_by_user_id UUID,
  received_by_user_id UUID,
  received_by_name TEXT,
  expected_return_at TIMESTAMPTZ,
  signed_out_at TIMESTAMPTZ,
  returned_at TIMESTAMPTZ,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE department_signout_items (
  id UUID PRIMARY KEY,
  signout_id UUID NOT NULL REFERENCES department_signouts(id) ON DELETE CASCADE,
  item_id UUID NOT NULL REFERENCES items(id),
  quantity_signed_out NUMERIC(12, 2) NOT NULL,
  quantity_returned NUMERIC(12, 2) NOT NULL DEFAULT 0,
  quantity_consumed NUMERIC(12, 2) NOT NULL DEFAULT 0,
  quantity_lost NUMERIC(12, 2) NOT NULL DEFAULT 0,
  quantity_damaged NUMERIC(12, 2) NOT NULL DEFAULT 0,
  condition_on_signout TEXT,
  condition_on_return TEXT,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inventory_movements (
  id UUID PRIMARY KEY,
  item_id UUID NOT NULL REFERENCES items(id),
  movement_type TEXT NOT NULL,
  from_location_id UUID REFERENCES locations(id),
  to_location_id UUID REFERENCES locations(id),
  from_department_id UUID REFERENCES departments(id),
  to_department_id UUID REFERENCES departments(id),
  related_signout_id UUID REFERENCES department_signouts(id),
  quantity NUMERIC(12, 2) NOT NULL,
  reason TEXT,
  created_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_log (
  id UUID PRIMARY KEY,
  actor_user_id UUID,
  action TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  before_json JSONB,
  after_json JSONB,
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 9. Availability calculation

The item detail page should show:

```text
Quantity owned
Quantity available
Quantity signed out
Quantity consumed
Quantity damaged
Quantity lost
Quantity reserved, future
```

For MVP, availability can be calculated as:

```text
quantity_available = quantity_owned - active_signed_out_quantity - damaged_quantity - lost_quantity
```

Where active signed out quantity is calculated from open sign-outs:

```text
quantity_signed_out - quantity_returned - quantity_consumed - quantity_lost - quantity_damaged
```

The exact calculation should be implemented in the backend service layer, not duplicated in the frontend.

## 10. Backend API endpoints

### Health

```text
GET /api/health
```

### Departments

```text
GET    /api/departments
POST   /api/departments
GET    /api/departments/{id}
PATCH  /api/departments/{id}
DELETE /api/departments/{id}
```

### Categories

```text
GET    /api/categories
POST   /api/categories
GET    /api/categories/{id}
PATCH  /api/categories/{id}
DELETE /api/categories/{id}
```

### Locations

```text
GET    /api/locations
POST   /api/locations
GET    /api/locations/{id}
PATCH  /api/locations/{id}
DELETE /api/locations/{id}
```

### Items

```text
GET    /api/items
POST   /api/items
GET    /api/items/{id}
PATCH  /api/items/{id}
POST   /api/items/{id}/retire
POST   /api/items/{id}/move
POST   /api/items/{id}/adjust-quantity
GET    /api/items/{id}/availability
GET    /api/items/{id}/movements
```

### Files

```text
POST   /api/items/{id}/images
DELETE /api/items/{id}/images/{fileId}
POST   /api/items/{id}/attachments
DELETE /api/items/{id}/attachments/{fileId}
GET    /api/files/{fileId}/download
```

### Department sign-outs

```text
GET    /api/signouts
POST   /api/signouts
GET    /api/signouts/{id}
PATCH  /api/signouts/{id}
POST   /api/signouts/{id}/prepare
POST   /api/signouts/{id}/confirm
POST   /api/signouts/{id}/return
POST   /api/signouts/{id}/close
POST   /api/signouts/{id}/cancel
GET    /api/signouts/active
GET    /api/departments/{id}/signouts
GET    /api/items/{id}/signouts
```

### Audit log

```text
GET /api/audit-log
```

### Export

```text
GET /api/export/items.csv
GET /api/export/signouts.csv
```

## 11. Frontend pages

```text
/dashboard
/items
/items/new
/items/:id
/items/:id/edit
/departments
/locations
/categories
/signouts
/signouts/new
/signouts/:id
/signouts/:id/return
/audit-log
/settings
```

## 12. Frontend item form sections

The item create and edit form should be grouped clearly.

```text
Basic info
  Name
  Description
  Category
  Item type
  Tags

Quantity
  Quantity owned
  Unit
  Minimum stock

Department
  Managed by department
  Needed by department

Storage
  Location
  Container

Purchase info
  Purchase price
  Currency
  Purchase date
  Supplier
  Purchase URL
  Replacement cost

Media
  Item picture
  Receipt / invoice upload

Status
  Condition
  Status
  Notes
```

## 13. Frontend sign-out flow

### Create sign-out

Fields:

```text
Destination department
Source department, optional
Source location, optional
Purpose
Expected return date
Items and quantities
Notes
```

The UI must check availability before confirming.

### Confirm sign-out

Fields:

```text
Signed out by
Received by
Confirmation notes
```

After confirmation, status becomes:

```text
Signed out
```

### Return sign-out

For each item line:

```text
Returned quantity
Consumed quantity
Damaged quantity
Lost quantity
Condition on return
Notes
```

The UI should make it easy to partially return a sign-out.

## 14. MVP scope

The MVP should include:

```text
PostgreSQL database
S3-compatible file storage through MinIO in development
Backend API in Go
Frontend in React TypeScript
Department management
Category management
Location management
Item management
Item image upload
Purchase information fields
Department sign-out creation
Sign-out confirmation
Partial and full return workflow
Availability calculation
Audit log
CSV export
Docker Compose development environment
```

Do not include in MVP:

```text
OIDC login
QR scanning
Offline PWA mode
Full purchase order approval workflow
Advanced reporting
Label printing
```

These should be designed for, but implemented later.

## 15. Version roadmap

### Version 0.1, foundation

```text
Project skeleton
Docker Compose
PostgreSQL
MinIO
Backend health endpoint
Database migrations
Frontend shell
```

### Version 0.2, base inventory

```text
Departments
Categories
Locations
Items
Item image upload
Purchase fields
Item list and detail pages
```

### Version 0.3, sign-out workflow

```text
Create sign-out
Add items to sign-out
Check availability
Confirm sign-out
Show active sign-outs
Return items
Mark consumed, damaged, or lost
```

### Version 0.4, operational polish

```text
Audit log
CSV export
Dashboard
Filters
Search
Basic role structure
Better error handling
```

### Version 1.0, production usable

```text
OIDC auth
Group based permissions
QR labels
QR scanning
Event packing lists
Low stock warnings
Backup and restore documentation
Kubernetes deployment manifests
Argo CD deployment setup
```

## 16. Docker Compose development services

Development should include:

```text
PostgreSQL
MinIO
Backend API
Frontend dev server
```

Environment variables should include:

```text
DATABASE_URL
S3_ENDPOINT
S3_BUCKET
S3_ACCESS_KEY_ID
S3_SECRET_ACCESS_KEY
S3_REGION
S3_USE_PATH_STYLE
APP_ENV
APP_BASE_URL
```

For local MinIO:

```text
S3_ENDPOINT=http://minio:9000
S3_BUCKET=awoostock-dev
S3_REGION=us-east-1
S3_USE_PATH_STYLE=true
```

## 17. Cursor implementation prompt

Use the following as the main Cursor instruction.

```text
We are building Stash, an internal inventory management system for Awoostria.

Stash manages convention equipment, consumables, containers, storage locations, department handovers, item images, purchase information, and returns.

Use the Vinuja project as the style reference for the Go codebase. The implementation should have a similar clean, practical Go structure, with small focused packages, explicit configuration, clear services, clear repositories, and readable production-minded code. Do not over-engineer the architecture, but keep it structured enough to grow.

The backend must be written in Go using Chi as the router, PostgreSQL as the database, pgx as the PostgreSQL driver, sqlc for typed SQL queries, and Goose for migrations. Business logic must live in service packages, not directly in HTTP handlers.

The frontend must be written in React with TypeScript using Vite, TanStack Router, TanStack Query, React Hook Form, Zod, Tailwind CSS, shadcn/ui, and Lucide icons.

Use PostgreSQL for structured data. Use S3-compatible object storage for files. In local development, use MinIO through Docker Compose. Do not store images, receipts, invoices, or manuals directly in PostgreSQL. Store only file metadata in PostgreSQL and the actual objects in S3-compatible storage.

The MVP must include:

1. Department management
2. Category management
3. Location management with parent-child hierarchy
4. Item management
5. Item image upload
6. Purchase information fields on items
7. Department sign-out creation
8. Sign-out confirmation
9. Partial and full return workflow
10. Consumed, damaged, and lost quantity tracking
11. Item availability calculation
12. Audit logging
13. CSV export
14. Docker Compose development environment with PostgreSQL and MinIO

Items must support:

- name
- description
- category
- item type, asset / consumable / container
- quantity owned
- unit
- condition
- status
- location
- container
- managed by department
- needed by department
- serial number
- asset tag
- owner
- purchase price
- purchase currency
- purchase date
- supplier
- purchase URL
- replacement cost
- minimum quantity
- primary image
- notes

The sign-out system must track temporary handover of inventory from storage or one department to another department. A sign-out represents transfer of responsibility, not permanent ownership.

A sign-out must include:

- sign-out number
- source department, optional
- source location, optional
- destination department
- purpose
- expected return date
- signed out by user
- received by user or free-text receiver name
- status
- notes
- line items with item, quantity signed out, quantity returned, quantity consumed, quantity lost, quantity damaged, condition on sign-out, condition on return, and notes

The backend must expose REST JSON APIs with consistent response and error formats. Every important state change must write an audit log entry. The frontend should be clean, practical, and optimized for fast inventory work during event preparation.

Start by generating the monorepo structure, Docker Compose setup, database migrations, backend skeleton, and the first API endpoints for departments, categories, locations, and items.
```

## 18. Key implementation rules

```text
Do not overwrite history. Use movement records and audit logs.
Do not store files in PostgreSQL. Store files in S3-compatible storage.
Do not hardcode departments. Departments are database records.
Do not make sign-outs permanent ownership transfers. They are temporary responsibility transfers.
Do not duplicate availability calculations in the frontend. Calculate in backend services.
Do not build QR scanning before the basic sign-out workflow works.
Do not build OIDC before the MVP data model and workflow are stable.
```

## 19. First implementation order

```text
1. Create monorepo structure
2. Create Docker Compose with PostgreSQL and MinIO
3. Add backend config loading
4. Add database connection
5. Add Goose migrations
6. Add sqlc configuration
7. Implement departments
8. Implement categories
9. Implement locations
10. Implement items
11. Implement S3 file upload for item images
12. Implement sign-outs
13. Implement return workflow
14. Implement availability calculation
15. Implement audit log
16. Implement CSV export
17. Build frontend layout
18. Build item list and item form
19. Build sign-out flow
20. Build dashboard
```

