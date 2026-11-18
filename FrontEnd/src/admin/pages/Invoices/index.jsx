import { useMemo, useState } from 'react';
import { Clock, CreditCard, Eye, Receipt, Wallet } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import SearchInput from '../../components/common/SearchInput.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import StatCard from '../../components/dashboard/StatCard.jsx';
import { PaymentStatusBadge } from '../../components/common/StatusBadge.jsx';
import { DataState } from '../../components/common/StateViews.jsx';
import InvoiceDetailModal from './InvoiceDetailModal.jsx';
import useCrud from '../../hooks/useCrud.js';
import { useToast } from '../../../shared/contexts/ToastContext.jsx';
import invoiceService, { invoiceTotal } from '../../../shared/services/invoiceService.js';
import activityService from '../../services/activityService.js';
import { PAYMENT_METHODS, PAYMENT_STATUS_LIST } from '../../../shared/utils/constants.js';
import { formatCurrency, formatDate } from '../../../shared/utils/formatters.js';
import { matchesSearch } from '../../utils/text.js';

export default function Invoices() {
  const toast = useToast();
  const { items, loading, error, reload, update } = useCrud(invoiceService);

  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [method, setMethod] = useState('');
  const [detail, setDetail] = useState(null);

  const filtered = useMemo(
    () =>
      items.filter(
        (inv) =>
          matchesSearch(inv, ['id', 'customerName', 'roomCode', 'bookingId'], search) &&
          (!status || inv.paymentStatus === status) &&
          (!method || inv.paymentMethod === method),
      ),
    [items, search, status, method],
  );

  const stats = useMemo(() => {
    const paid = items.filter((i) => i.paymentStatus === 'Đã thanh toán');
    const unpaid = items.filter((i) => i.paymentStatus === 'Chưa thanh toán');
    const sum = (list) => list.reduce((s, i) => s + invoiceTotal(i), 0);
    return { paid: sum(paid), paidCount: paid.length, unpaid: sum(unpaid), unpaidCount: unpaid.length };
  }, [items]);

  const hasFilter = Boolean(search || status || method);

  const handlePay = async (invoice, paymentMethod) => {
    try {
      const updated = await update(invoice.id, {
        paymentStatus: 'Đã thanh toán',
        paymentMethod,
        paidAt: new Date().toISOString(),
      });
      activityService.log('payment', `Thanh toán hóa đơn ${invoice.id} – ${invoice.customerName}`);
      toast.success(`Đã ghi nhận thanh toán hóa đơn ${invoice.id}`);
      setDetail(updated);
    } catch (e) {
      toast.error(e.message);
    }
  };

  const columns = [
    { key: 'id', header: 'Mã HĐ', render: (i) => <span className="font-semibold text-navy-800">{i.id}</span> },
    {
      key: 'customerName',
      header: 'Khách hàng',
      render: (i) => (
        <div>
          <p className="font-medium text-slate-800">{i.customerName}</p>
          <p className="text-xs text-slate-400">
            {i.bookingId} · Phòng {i.roomCode}
          </p>
        </div>
      ),
    },
    { key: 'createdAt', header: 'Ngày lập', hideBelow: 'lg', render: (i) => formatDate(i.createdAt) },
    { key: 'roomCharge', header: 'Tiền phòng', hideBelow: 'xl', className: 'text-right', render: (i) => formatCurrency(i.roomCharge) },
    { key: 'serviceCharge', header: 'Dịch vụ', hideBelow: 'xl', className: 'text-right', render: (i) => formatCurrency(i.serviceCharge) },
    { key: 'surcharge', header: 'Phụ thu', hideBelow: 'xl', className: 'text-right', render: (i) => formatCurrency(i.surcharge) },
    { key: 'total', header: 'Tổng tiền', className: 'text-right', render: (i) => <span className="font-semibold text-navy-900">{formatCurrency(invoiceTotal(i))}</span> },
    { key: 'paymentMethod', header: 'Thanh toán', hideBelow: 'md' },
    { key: 'paymentStatus', header: 'Trạng thái', render: (i) => <PaymentStatusBadge status={i.paymentStatus} /> },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (i) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <button type="button" className="btn-icon" title="Xem chi tiết / In" onClick={() => setDetail(i)}>
            <Eye size={16} />
          </button>
          {i.paymentStatus === 'Chưa thanh toán' && (
            <button type="button" className="btn-icon text-amber-600" title="Thanh toán" onClick={() => setDetail(i)}>
              <CreditCard size={16} />
            </button>
          )}
        </div>
      ),
    },
  ];

  return (
    <div>
      <PageHeader title="Hóa đơn & Thanh toán" description={`${items.length} hóa đơn`} />

      <div className="mb-5 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard title="Tổng hóa đơn" value={items.length} subtitle="Tất cả trạng thái" icon={Receipt} tone="navy" />
        <StatCard title="Đã thu" value={formatCurrency(stats.paid)} subtitle={`${stats.paidCount} hóa đơn`} icon={Wallet} tone="green" />
        <StatCard title="Chưa thu" value={formatCurrency(stats.unpaid)} subtitle={`${stats.unpaidCount} hóa đơn`} icon={Clock} tone="amber" />
      </div>

      <div className="card overflow-hidden">
        <div className="grid grid-cols-1 gap-3 border-b border-slate-100 p-4 sm:grid-cols-2 xl:grid-cols-4">
          <SearchInput value={search} onChange={setSearch} placeholder="Tìm mã HĐ, khách hàng, phòng..." className="sm:col-span-2" />
          <select className="input" value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Lọc theo trạng thái">
            <option value="">Tất cả trạng thái</option>
            {PAYMENT_STATUS_LIST.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
          <select className="input" value={method} onChange={(e) => setMethod(e.target.value)} aria-label="Lọc theo phương thức">
            <option value="">Mọi phương thức</option>
            {PAYMENT_METHODS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </div>

        <DataState
          loading={loading}
          error={error}
          onRetry={reload}
          isEmpty={filtered.length === 0}
          emptyProps={{
            icon: Receipt,
            title: hasFilter ? 'Không tìm thấy hóa đơn' : 'Chưa có hóa đơn',
            description: hasFilter ? 'Thử thay đổi từ khóa hoặc bộ lọc.' : 'Hóa đơn được tạo khi khách check-out.',
            action: hasFilter ? (
              <button
                type="button"
                className="btn-secondary"
                onClick={() => {
                  setSearch('');
                  setStatus('');
                  setMethod('');
                }}
              >
                Xóa bộ lọc
              </button>
            ) : undefined,
          }}
        >
          <DataTable columns={columns} data={filtered} onRowClick={setDetail} />
        </DataState>
      </div>

      <InvoiceDetailModal invoice={detail} onClose={() => setDetail(null)} onPay={handlePay} />
    </div>
  );
}
