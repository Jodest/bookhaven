import { api } from '@bookhaven/api-client';

export const adminHarvestApi = {
  trigger: api.admin.harvest.trigger,
  listJobs: api.admin.harvest.jobs.list,
  getJob: api.admin.harvest.jobs.get,
};
