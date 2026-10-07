import { api } from '@bookhaven/api-client';

export const adminDashboardApi = {
  get: api.admin.dashboard,
};

// Query key of the admin dashboard stats (GET /admin/dashboard). Shared by the
// dashboard page and the sidebar/tab counters - always go through
// useDashboardStats so the key maps to a single queryFn/data shape.
export const DASHBOARD_QUERY_KEY = ['admin', 'dashboard'];
