# Business-domain analysis

## Ubiquitous terms and lifecycle

- **Room type** is sellable accommodation specification/content and base price; **room** is a physical unit.
- **Booking group** is one customer submission for one or more rooms; **reservation** is the allocated physical room for its stay interval.
- **Stay** records actual occupancy; an invoice is created at checkout and is not a mutable reservation total.
- **Customer** is the guest/person record; a customer login is an identity linked one-to-one to a customer profile. A staff login links to an employee record.

| Aggregate | State/lifecycle invariant | Owner |
|---|---|---|
| User | active → suspended/disabled; credentials never returned; sessions revoked on disable/password reset | Identity & Access |
| Customer/employee | profile may be archived, never hard-deleted while referenced | People |
| Room | available, reserved, occupied, cleaning, maintenance, out_of_service; manual state cannot contradict active stay | Property |
| Reservation | pending → confirmed → checked_in → checked_out; pending/confirmed → cancelled; `no_show` is later policy. Terminal states are immutable except correction workflows. | Reservations / Stay Operations |
| Booking group | active until all child reservations terminal; group cancellation rules must be explicit | Reservations |
| Invoice | issued → partially_paid/paid/refunded/void (void policy unresolved); issue snapshot/lines immutable | Billing |
| Support ticket | open → in_progress → resolved/closed; messages append-only | Support |

## Business invariants

1. A physical room cannot have overlapping active (`pending`, `confirmed`, `checked_in`) stay intervals. PostgreSQL is the final arbiter.
2. Date interval is `[check_in_date, check_out_date)`, checkout must follow checkin, and capacity must fit booked guests under confirmed policy.
3. A reservation price snapshot is computed by the server at creation. Altering a room type price, tax policy, service price, or description does not change it.
4. Only confirmed reservations can check in; only checked-in reservations can check out. Checkout atomically creates one invoice and moves room to cleaning.
5. Service invoice lines retain service name/price/tax snapshot even if the service is later modified/deactivated.
6. Deactivation/archive replaces destructive deletion for users, roles, rooms, room types, services, and referenced people. Legal/accounting records are retained.
7. Customer access is contextual: the authenticated customer can view/cancel only reservations whose group belongs to their customer profile; staff permissions do not grant arbitrary customer ownership.

## Coordination and consistency

The Reservation application service owns group creation and invokes read ports from Property/People; it writes only reservation tables. Room assignment is protected by a transaction, `SELECT ... FOR UPDATE SKIP LOCKED` candidate selection where useful, and the exclusion constraint. The database constraint is mandatory because allocation can race.

Stay Operations coordinates checkout by calling Billing’s `IssueFromCheckout` interface in the same transaction supplied by the transaction runner. Billing owns invoice tables; Property owns room operational state. Exposed interfaces are application methods, not table/repository exposure. Audit/outbox messages are committed in that transaction.

## Reporting scope

Dashboard/reports read existing reservations, stays, invoices, payments and service lines. Initial reports are SQL read models/materialized queries owned by Reporting. Do not add summary tables until scale evidence requires daily snapshots. Definitions requiring stakeholder confirmation: revenue recognition date (checkout vs payment), cancellation/refund treatment, occupancy denominator, and customer “returning” definition.
