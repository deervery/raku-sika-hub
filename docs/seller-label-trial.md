# Optional seller row: office trial

The HTTP print/preview API accepts `data.sellerBlock` (name, address, optional phone; 2–3 nonempty lines, at most 800 characters) only when `HUB_SELLER_LABEL_TRIAL=1`.
The default is disabled. Enabled instances advertise `seller-label-ja-traceable-v1` in `/version`.

Only Japanese traceable meat templates support the row. Other templates and the English layout reject requests containing a seller rather than silently dropping it.
The seller is a new row, independent of company/processing facility. It wraps the full value and grows the row without truncation.
Without a seller, existing rows and rendering remain unchanged.

Office verification uses a separate temporary instance on loopback port 19801. The normal service on 19800, its configuration, USB scale, scanner, and deployed binary are unchanged.
The trial frontend is enabled only for an isolated facility in the Lite development project. No production release/tag is created for this trial.

Tests cover the disabled feature, advertised capability, template/language scope, unchanged existing rows, and long addresses.
On office, 12 seller-free previews from the old and trial instances matched byte-for-byte (meat JA/EN, non-traceable meat, processed, pet, carcass, and individual QR).
