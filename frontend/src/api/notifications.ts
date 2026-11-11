import { apiRequest } from './client';

export type OverdueSignout = {
  id: string;
  signout_number: string;
  to_department_name: string;
  received_by_name: string;
  expected_return_at: string;
};

export type Notifications = {
  /** Empty for people without signout.view. */
  overdue_signouts: OverdueSignout[];
};

export const getNotifications = () => apiRequest<Notifications>('/api/notifications');
