import { createMockService } from './createMockService.js';
import { seedInvoices } from './mockData.js';

export const invoiceTotal = (inv) =>
  Number(inv.roomCharge || 0) + Number(inv.serviceCharge || 0) + Number(inv.surcharge || 0);

const invoiceService = createMockService({
  prefix: 'HD',
  pad: 4,
  initial: seedInvoices,
  sortBy: (a, b) => (b.createdAt || '').localeCompare(a.createdAt || ''),
});

export default invoiceService;
