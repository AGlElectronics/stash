export type Department = {
  id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
};

export type Category = {
  id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
};

export type Location = {
  id: string;
  parent_id?: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
};

export type ItemType = 'asset' | 'consumable' | 'container' | 'grouped';

export type Item = {
  id: string;
  name: string;
  description: string;
  category_id?: string;
  item_type: ItemType;
  size?: string;
  quantity_owned: number;
  quantity_damaged: number;
  quantity_lost: number;
  unit: string;
  condition: string;
  status: 'available' | 'in_use' | 'retired';
  location_id?: string;
  parent_container_id?: string;
  parent_group_id?: string;
  managed_by_department_id?: string;
  needed_by_department_id?: string;
  serial_number?: string;
  asset_tag?: string;
  qr_code?: string;
  owner: string;
  purchase_price?: string;
  purchase_currency: string;
  purchase_date?: string;
  expiry_date?: string;
  supplier?: string;
  purchase_url?: string;
  replacement_cost?: string;
  minimum_quantity: number;
  primary_image_file_id?: string;
  notes: string;
  created_at: string;
  updated_at: string;
  retired_at?: string;
};

export type ItemAvailability = {
  quantity_owned: number;
  quantity_damaged: number;
  quantity_lost: number;
  quantity_signed_out: number;
  quantity_available: number;
};

export type ItemEventType =
  | 'created'
  | 'moved'
  | 'quantity_adjusted'
  | 'signed_out'
  | 'returned'
  | 'consumed'
  | 'marked_damaged'
  | 'repaired'
  | 'marked_lost'
  | 'marked_found'
  | 'condition_changed'
  | 'retired'
  | 'restored';

export type ItemEvent = {
  id: string;
  item_id: string;
  event_type: ItemEventType;
  quantity: number;
  from_location_id?: string;
  to_location_id?: string;
  from_department_id?: string;
  to_department_id?: string;
  related_signout_id?: string;
  related_event_id?: string;
  related_file_id?: string;
  condition_before?: string;
  condition_after?: string;
  actor_user_id?: string;
  actor_oidc_subject?: string;
  actor_display_name?: string;
  note: string;
  occurred_at: string;
};

export type Me = {
  id: string;
  display_name: string;
  email: string;
  oidc_subject: string;
  /** Global role keys, e.g. "stash_volunteer". */
  roles: string[];
  permissions: string[];
};

export type Paginated<T> = {
  items: T[];
  total: number;
};

export type EventList = {
  events: ItemEvent[];
  total: number;
};

export type SignoutStatus =
  | 'draft'
  | 'prepared'
  | 'signed_out'
  | 'partially_returned'
  | 'returned'
  | 'closed'
  | 'cancelled';

export type Signout = {
  id: string;
  signout_number: string;
  from_department_id?: string;
  to_department_id: string;
  from_location_id?: string;
  status: SignoutStatus;
  purpose: string;
  signed_out_by_user_id?: string;
  received_by_user_id?: string;
  received_by_name?: string;
  expected_return_at?: string;
  signed_out_at?: string;
  returned_at?: string;
  closed_at?: string;
  cancelled_at?: string;
  notes: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
};

export type SignoutItem = {
  id: string;
  signout_id: string;
  item_id: string;
  quantity_signed_out: number;
  quantity_returned: number;
  quantity_consumed: number;
  quantity_lost: number;
  quantity_damaged: number;
  condition_on_signout?: string;
  condition_on_return?: string;
  notes: string;
  created_at: string;
  updated_at: string;
};

export type SignoutWithItems = Signout & { items: SignoutItem[] };
