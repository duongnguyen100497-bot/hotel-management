import { createMockService } from '../../shared/services/createMockService.js';
import { seedActivities } from '../../shared/services/mockData.js';

const base = createMockService({
  prefix: 'AC',
  pad: 3,
  initial: seedActivities,
  sortBy: (a, b) => b.time.localeCompare(a.time),
});

const activityService = {
  ...base,
  async getRecent(limit = 8) {
    const all = await base.getAll();
    return all.slice(0, limit);
  },
  // type: checkin | checkout | booking | payment | room
  log(type, message) {
    return base.insertSync({ type, message, time: new Date().toISOString() });
  },
};

export default activityService;
