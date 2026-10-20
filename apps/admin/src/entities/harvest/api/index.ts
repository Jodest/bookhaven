import { api } from '@bookhaven/api-client';

export const adminHarvestApi = {
  trigger: api.admin.harvest.trigger,
  listJobs: api.admin.harvest.jobs.list,
  getJob: api.admin.harvest.jobs.get,
  searchAuthors: api.admin.harvest.authors.search,
  triggerCatalogAdaptationSync: () => api.admin.books.triggerCatalogAdaptationSync(),
};

// Query key of the harvest jobs list (GET /admin/harvest/jobs).
export const HARVEST_JOBS_QUERY_KEY = ['admin', 'harvest', 'jobs'];
