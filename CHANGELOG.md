# Changelog

## [0.1.1](https://github.com/bbmumford/entitlement/compare/v0.1.0...v0.1.1) (2026-11-28)


### Bug Fixes

* a feature set never confers a member whose own release flag is off ([1828341](https://github.com/bbmumford/entitlement/commit/18283414b5f38de8a8ce81198919d91eb1faeb97))
* a feature set with no release flag is on whatever flagOn says ([9c9f5c2](https://github.com/bbmumford/entitlement/commit/9c9f5c211b15bd7d68db2aab90a4effb0cab920c))
* a wired invite resolver gates only the records marked InviteOnly ([0d1151c](https://github.com/bbmumford/entitlement/commit/0d1151cc2692d4c65ab33d02f483db92cdb29638))
* an invite-only surface whose invite resolver is not wired admits no one ([fa146b7](https://github.com/bbmumford/entitlement/commit/fa146b7398eca553dbd8ee86fef3d1c17cf9aae7))
* the registry shares no slice or map with its callers ([b8421a7](https://github.com/bbmumford/entitlement/commit/b8421a70d95deb7ee6c74ed96b40e944541c8e07))
* Validate refuses a count below -1 ([5515d38](https://github.com/bbmumford/entitlement/commit/5515d385b4e7ab53158b65e8862a98182eb4d863))

## [0.1.0](https://github.com/bbmumford/entitlement/compare/v0.0.2...v0.1.0) (2026-11-25)


### Features

* a feature key carries named parameters, each a set of values or a count per plan ([33d7e41](https://github.com/bbmumford/entitlement/commit/33d7e41673501440828801624b3ea031523c931f))
* a registry checks each namespace's rows against the plans its own product declares ([b36d0d5](https://github.com/bbmumford/entitlement/commit/b36d0d571c5b9b0ec838114ee4bfe285df7e053f))
* the gate names the reason for each refusal ([29f44e1](https://github.com/bbmumford/entitlement/commit/29f44e18118f2dd397306b0c0c95542663ca0529))


### Bug Fixes

* a feature set with no members entitles no plan ([2793e4b](https://github.com/bbmumford/entitlement/commit/2793e4b17aabc0f59e0f75cfa565465eaf7b0a67))
