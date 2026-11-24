# Integration and infrastructure design

| Integration | Evidence / owner | Initial? | Design and failure handling |
|---|---|---:|---|
| Transactional email | Frontend promises booking confirmation/contact/password reset but sends no real email; Identity, Reservations, Support | Yes | Outbox event after commit; worker retries exponential with cap/dead-letter alert. Templates use snapshots and no sensitive document data. Provider decision open. |
| Object storage + image processing | hotel logo upload and static room/gallery images; Property | Yes for editable media | private bucket, presigned upload, scan/validate, store metadata; CDN/public published derivatives only. |
| Payment provider | card payment explicitly disabled; Billing | No | Keep manual cash/transfer payment recording. Add provider adapter/webhook signature/idempotency only after provider and refund/settlement policy are selected. |
| Report export | UI exports CSV/prints PDF; Reporting | Yes | CSV server streaming for bounded results; PDF stays client print initially. Large exports become outbox/job + short-lived download. |
| Real-time / WebSocket | No evidence | No | Poll/refetch after commands. Add SSE for dashboard/room boards only if latency/user-count evidence justifies it. |
| SMS, social, P2P/STUN/TURN | No evidence | No | explicitly excluded. |

The outbox worker runs in the same deployable initially. It claims events with `FOR UPDATE SKIP LOCKED`, records delivery attempts, is idempotent at consumers/providers, and emits metrics. There is no broker requirement now.
