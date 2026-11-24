# Third-party notices

| Field | Value |
|---|---|
| Repository | `digital.vasic.containers` |
| Revision | measured 2026-11-24 against the `go.mod` files named below |
| Referenced by | `LICENSE` section 4 |
| Gate | `scripts/verify_licence.sh` requires this file |

`LICENSE` section 4 states that third-party components remain under their own licences,
listed in this file. This file lists them. It does not grant, change or restrict any right
under those licences.

## Scope and method

- **Nothing is vendored.** This repository contains no copied third-party source tree
  (`vendor/`, `third_party/`, `node_modules/`) and no git submodules. Third-party Go modules
  are resolved at build time from `go.mod` / `go.sum`. Every file in the repository that
  carries a copyright line names the repository owner.
- **Source of the list:** the `require` blocks of the `go.mod` named below, direct and
  indirect. Own-organisation modules (`digital.vasic.*`) are not third-party and are listed
  separately.
- **How each licence was determined:** the module at the pinned version was downloaded with
  `go mod download` and its licence file read; the licence name was assigned from the licence
  text. A row says UNKNOWN where no licence file could be read.
- **Not covered:** the Go standard library and toolchain (distributed under the Go
  project's BSD-3-Clause licence, not copied into this repository) and tools a developer may
  install separately.

## Go modules required by `go.mod`

| Module | Version | Relation | Licence | Licence file |
|---|---|---|---|---|
| `github.com/prometheus/client_golang` | `v1.23.2` | direct | Apache-2.0 | LICENSE +NOTICE |
| `github.com/prometheus/client_model` | `v0.6.2` | direct | Apache-2.0 | LICENSE +NOTICE |
| `github.com/stretchr/testify` | `v1.11.1` | direct | MIT | LICENSE |
| `golang.org/x/crypto` | `v0.52.0` | direct | BSD-3-Clause | LICENSE |
| `golang.org/x/term` | `v0.43.0` | direct | BSD-3-Clause | LICENSE |
| `gopkg.in/yaml.v3` | `v3.0.1` | direct | MIT AND Apache-2.0 (MIT covers the files ported from libyaml; Apache-2.0 the rest) | LICENSE +NOTICE |
| `github.com/beorn7/perks` | `v1.0.1` | indirect | MIT | LICENSE |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | indirect | MIT | LICENSE.txt |
| `github.com/davecgh/go-spew` | `v1.1.2-0.20180830191138-d8f796af33cc` | indirect | ISC | LICENSE |
| `github.com/kr/text` | `v0.2.0` | indirect | MIT | License (read manually) |
| `github.com/munnerz/goautoneg` | `v0.0.0-20191010083416-a7dc8b61c822` | indirect | BSD-3-Clause | LICENSE |
| `github.com/pmezard/go-difflib` | `v1.0.1-0.20181226105442-5d4384ee4fb2` | indirect | BSD-3-Clause | LICENSE |
| `github.com/prometheus/common` | `v0.67.5` | indirect | Apache-2.0 | LICENSE +NOTICE |
| `github.com/prometheus/procfs` | `v0.20.1` | indirect | Apache-2.0 | LICENSE +NOTICE |
| `github.com/rogpeppe/go-internal` | `v1.14.1` | indirect | BSD-3-Clause | LICENSE |
| `go.yaml.in/yaml/v2` | `v2.4.4` | indirect | MIT AND Apache-2.0 (MIT covers the files ported from libyaml, see LICENSE.libyaml) | LICENSE +NOTICE +LICENSE.libyaml |
| `golang.org/x/sys` | `v0.45.0` | direct | BSD-3-Clause | LICENSE |
| `google.golang.org/protobuf` | `v1.36.11` | indirect | BSD-3-Clause | LICENSE |

## Items that could not be verified

None of the modules above is UNKNOWN. Licence names were assigned from licence text
and not from legal review; consult each module's own licence file for the binding terms.
