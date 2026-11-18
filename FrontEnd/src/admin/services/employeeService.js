import { createMockService } from '../../shared/services/createMockService.js';
import { seedEmployees } from '../../shared/services/mockData.js';

const employeeService = createMockService({
  prefix: 'NV',
  pad: 3,
  initial: seedEmployees,
  unique: [
    { field: 'username', message: 'Tên tài khoản đã tồn tại' },
    { field: 'email', message: 'Email đã được sử dụng' },
  ],
  sortBy: (a, b) => a.id.localeCompare(b.id),
});

export default employeeService;
