package seed

import _ "embed"

// RBACSeedSQL contains the idempotent SQL statements that seed all canonical
// system permissions and the 4 default roles (system-administrator, hotel-manager,
// front-desk, accounting).
//
//go:embed rbac_seed.sql
var RBACSeedSQL string
