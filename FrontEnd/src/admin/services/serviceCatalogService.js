import { createMockService } from '../../shared/services/createMockService.js';
import { seedServices } from '../../shared/services/mockData.js';

// Dịch vụ khách sạn (giặt ủi, spa, minibar...).
const serviceCatalogService = createMockService({
  prefix: 'DV',
  pad: 3,
  initial: seedServices,
  unique: [{ field: 'name', message: 'Tên dịch vụ đã tồn tại' }],
  sortBy: (a, b) => a.id.localeCompare(b.id),
});

export default serviceCatalogService;
