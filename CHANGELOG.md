# Changelog

## [1.1.0](https://github.com/koment-dev/koment/compare/v1.0.0...v1.1.0) (2026-08-06)


### Features

* **container:** update image curlimages/curl (8.16.0 → 8.21.0) ([#55](https://github.com/koment-dev/koment/issues/55)) ([4c9f9c5](https://github.com/koment-dev/koment/commit/4c9f9c5f05f08d267284ab515c1dbc5e490deec2))
* honour the rate limit github reports on every response ([#62](https://github.com/koment-dev/koment/issues/62)) ([fe89381](https://github.com/koment-dev/koment/commit/fe89381a1842de8553d264155bd3d9fa1d6e0b37))
* **npm:** update dependency ovsx (1.0.2 → 1.1.0) ([#56](https://github.com/koment-dev/koment/issues/56)) ([dbcfbd2](https://github.com/koment-dev/koment/commit/dbcfbd25deeea53342c041d013dce401ce8a0bdc))


### Bug Fixes

* close the code findings codeql reported ([#65](https://github.com/koment-dev/koment/issues/65)) ([c850130](https://github.com/koment-dev/koment/commit/c850130f1435fbaa0b6d621dc80473e662b548ae))
* give renovate the commit-status permission it aborts without ([#51](https://github.com/koment-dev/koment/issues/51)) ([1cd1216](https://github.com/koment-dev/koment/commit/1cd121655b9c7b00ae7f263108a61caed55c533f))
* pin codeql-action to the version its comment claims ([#50](https://github.com/koment-dev/koment/issues/50)) ([14bec7e](https://github.com/koment-dev/koment/commit/14bec7ede6ef744b2b141f67bdc7554d9157b0fa))
* stop renovate raising the extension's supported editor floor ([#63](https://github.com/koment-dev/koment/issues/63)) ([c8e2171](https://github.com/koment-dev/koment/commit/c8e2171b29b48555b41bc688877258e6df6fe47a))
* stop renovate rewriting the hand-written chart schema ([#60](https://github.com/koment-dev/koment/issues/60)) ([564b51b](https://github.com/koment-dev/koment/commit/564b51b38a0ff4583bb08e564eca792c10cf640d))


### Documentation

* make the licence identifiable and the badges truthful ([#61](https://github.com/koment-dev/koment/issues/61)) ([7327899](https://github.com/koment-dev/koment/commit/7327899e6b41e8c138bbf987632fafdc055f12e6))
* record what the renovate investigation cost ([#52](https://github.com/koment-dev/koment/issues/52)) ([76d6acc](https://github.com/koment-dev/koment/commit/76d6acc87b66d5b4bc89cc11b7b1a66966a24b36))


### Continuous Integration

* **github-action:** Update action renovatebot/github-action (v46.2.0 → v46.2.1) ([#66](https://github.com/koment-dev/koment/issues/66)) ([a21ee5e](https://github.com/koment-dev/koment/commit/a21ee5e2c816bcecfbef2b86876f06bba33e0091))
* **github-action:** Update github-actions ([#54](https://github.com/koment-dev/koment/issues/54)) ([1b52d8e](https://github.com/koment-dev/koment/commit/1b52d8e4ffd754309813e50639c211c48bca4661))
* **github-action:** Update github-actions (major) ([#58](https://github.com/koment-dev/koment/issues/58)) ([ade569a](https://github.com/koment-dev/koment/commit/ade569a7317057aa204300e17eb2db9a5b72452a))
* let renovate run the chart post-upgrade tasks ([#49](https://github.com/koment-dev/koment/issues/49)) ([011e5ef](https://github.com/koment-dev/koment/commit/011e5ef24930af7cec15713cc46796165f0400b6))
* run renovate on github runners ([#47](https://github.com/koment-dev/koment/issues/47)) ([694e666](https://github.com/koment-dev/koment/commit/694e666f353ab8b98f7916945df6bfcea4458beb))

## [1.0.0](https://github.com/koment-dev/koment/compare/v0.8.0...v1.0.0) (2026-08-05)


### ⚠ BREAKING CHANGES

* annotation records adopt the Kubernetes shape (apiVersion: koment.dev/v1alpha, kind: Annotation, metadata/spec/status) and .koment/policy.yaml adopts kind: Policy in the same group. The rationale category moves from `kind` to `spec.type`, `anchor.last_seen_line` moves to `status.lastSeenLine`, `git.end_line` becomes `git.endLine`, `created` becomes an RFC3339 instant, and the published schemas move to schema/v1alpha/. A flat `version: 1` file of either kind is rewritten in place the first time a 1.x binary reads it, so no repository needs a migration command; a 0.x binary cannot read a repository once that has happened. The MCP `koment_add` result replaces `record.version` with `record.api_version`; every other JSON surface is unchanged.

### Features

* reshape the annotation record and cut off v1 ([#45](https://github.com/koment-dev/koment/issues/45)) ([cd56145](https://github.com/koment-dev/koment/commit/cd56145629bbe01731622910d0d22f0cd16c2bc1))


### Bug Fixes

* title every annotation so all 70 records parse under v0.5.0+ ([#43](https://github.com/koment-dev/koment/issues/43)) ([5c7b809](https://github.com/koment-dev/koment/commit/5c7b8099c049c7ce4a5ca3458c254b0db84c00c9))

## [0.8.0](https://github.com/koment-dev/koment/compare/v0.7.0...v0.8.0) (2026-08-05)


### Features

* ship opencode as a first-class peer of codex ([#41](https://github.com/koment-dev/koment/issues/41)) ([f2a7a0e](https://github.com/koment-dev/koment/commit/f2a7a0ea37beaccbb6c4c042a5866fdd4330ab12))

## [0.7.0](https://github.com/koment-dev/koment/compare/v0.6.0...v0.7.0) (2026-08-05)


### ⚠ BREAKING CHANGES

* relicense to AGPL-3.0-or-later with commercial dual licensing ([#37](https://github.com/koment-dev/koment/issues/37))

### Features

* add CLA gate with in-repo signing workflow ([#39](https://github.com/koment-dev/koment/issues/39)) ([b0e03bb](https://github.com/koment-dev/koment/commit/b0e03bb7865dcf9dc7e3dbb59d3ec4e942734b49))
* relicense to AGPL-3.0-or-later with commercial dual licensing ([#37](https://github.com/koment-dev/koment/issues/37)) ([bca8d2e](https://github.com/koment-dev/koment/commit/bca8d2e0d2eb4c9c6e25b20f1153099b199cc9a9))

## [0.6.0](https://github.com/koment-dev/koment/compare/v0.5.1...v0.6.0) (2026-08-05)


### ⚠ BREAKING CHANGES

* stop reporting a stale line number as a status ([#35](https://github.com/koment-dev/koment/issues/35))

### Code Refactoring

* stop reporting a stale line number as a status ([#35](https://github.com/koment-dev/koment/issues/35)) ([7509a06](https://github.com/koment-dev/koment/commit/7509a06f617103bfa7a9bab3f7d9cebea92bf8d3))

## [0.5.1](https://github.com/koment-dev/koment/compare/v0.5.0...v0.5.1) (2026-08-05)


### Bug Fixes

* stop the install checks racing the release they verify ([#33](https://github.com/koment-dev/koment/issues/33)) ([54d9f40](https://github.com/koment-dev/koment/commit/54d9f40994b5407c07d7511bd20fc3e25b64d475))

## [0.5.0](https://github.com/koment-dev/koment/compare/v0.4.0...v0.5.0) (2026-08-05)


### Features

* give an annotation a title so nothing is shown cut off ([#31](https://github.com/koment-dev/koment/issues/31)) ([ce4725a](https://github.com/koment-dev/koment/commit/ce4725a0f199e7165753771470171af0a16e43a8))

## [0.4.0](https://github.com/koment-dev/koment/compare/v0.3.1...v0.4.0) (2026-08-04)


### Features

* give rationale a panel and stop marking healthy code ([#30](https://github.com/koment-dev/koment/issues/30)) ([e154ed6](https://github.com/koment-dev/koment/commit/e154ed6dd9078df0e266146a78c5536fe52a71d1))
* make the editor and the toolchain find koment ([#27](https://github.com/koment-dev/koment/issues/27)) ([e144e61](https://github.com/koment-dev/koment/commit/e144e614b44fe4848b0d29dc3aa1b0225e0c632c))


### Bug Fixes

* stop the language server putting null where LSP declares an array ([#28](https://github.com/koment-dev/koment/issues/28)) ([8b78d72](https://github.com/koment-dev/koment/commit/8b78d721a878a7b3d328fb18c334cd0362398b81))

## [0.3.1](https://github.com/koment-dev/koment/compare/v0.3.0...v0.3.1) (2026-08-04)


### Bug Fixes

* repair the three publish failures that emptied 0.3.0 ([#25](https://github.com/koment-dev/koment/issues/25)) ([4a4f530](https://github.com/koment-dev/koment/commit/4a4f530a7983cdd1c09c0403911f8a029cff0ebc))

## [0.3.0](https://github.com/koment-dev/koment/compare/v0.2.0...v0.3.0) (2026-08-04)


### Features

* add local writes and policy enforcement ([#20](https://github.com/koment-dev/koment/issues/20)) ([2829410](https://github.com/koment-dev/koment/commit/2829410405f9e1bbbdb797079601ab2ef24bcf9d))
* distribute koment through package managers, marketplaces and editors ([#21](https://github.com/koment-dev/koment/issues/21)) ([9d5f421](https://github.com/koment-dev/koment/commit/9d5f421337f9681286f43376d418a7732a204542))
* reset annotation records and anchors ([#19](https://github.com/koment-dev/koment/issues/19)) ([2084861](https://github.com/koment-dev/koment/commit/208486110e29da25e515684b3917ff94ce819bbf))


### Bug Fixes

* keep the chart README generatable across a version bump ([#22](https://github.com/koment-dev/koment/issues/22)) ([4139feb](https://github.com/koment-dev/koment/commit/4139feb2ae984c71422c4a4dc3ec18fe3a271b3d))
* stop deleting the helm test pod before its logs are read ([#23](https://github.com/koment-dev/koment/issues/23)) ([2ff1ed4](https://github.com/koment-dev/koment/commit/2ff1ed471065e23cd09057197ed8d26400b76168))


### Documentation

* reset architecture for vNext ([#17](https://github.com/koment-dev/koment/issues/17)) ([1d12296](https://github.com/koment-dev/koment/commit/1d122963f024f48ed04833c6148369f92bbdebbc))


### Continuous Integration

* exercise the setup action against a published release ([#15](https://github.com/koment-dev/koment/issues/15)) ([c5ad926](https://github.com/koment-dev/koment/commit/c5ad926365542b1db0ebec343978d5b9a5b3e21a))
* name every job after its id and stop paying for setup twice ([#24](https://github.com/koment-dev/koment/issues/24)) ([963b574](https://github.com/koment-dev/koment/commit/963b57417318160647448f2b81712db83ea5dcde))

## [0.2.0](https://github.com/koment-dev/koment/compare/v0.1.2...v0.2.0) (2026-08-03)


### Features

* .koment bootstraps the index, and export rebuilds .koment from it ([#9](https://github.com/koment-dev/koment/issues/9)) ([90357ea](https://github.com/koment-dev/koment/commit/90357ea3b697f1235c7937484010cf43dd1654ad))
* a nested file tree, a repository switcher, and notes that float ([#13](https://github.com/koment-dev/koment/issues/13)) ([725f526](https://github.com/koment-dev/koment/commit/725f526da5dbc756cae655edcdfb96f5650963cd))
* multi-repository is first class, not a second citizen ([#11](https://github.com/koment-dev/koment/issues/11)) ([5655d1b](https://github.com/koment-dev/koment/commit/5655d1b23479e361e9cc9d0358f0d0b0cbfb0249))
* publishing is a first-class tier, not demo scaffolding ([#12](https://github.com/koment-dev/koment/issues/12)) ([7da5802](https://github.com/koment-dev/koment/commit/7da5802d868b3cefd37043ef7f38b53095841088))

## [0.1.2](https://github.com/koment-dev/koment/compare/v0.1.1...v0.1.2) (2026-08-02)


### Features

* index annotations in a database; git keeps the record ([#8](https://github.com/koment-dev/koment/issues/8)) ([a28ac9d](https://github.com/koment-dev/koment/commit/a28ac9dc89791a5d2a493f0fd858c933da128e7e))
* metrics, env configuration, and a two-repository demo ([#6](https://github.com/koment-dev/koment/issues/6)) ([9fbd582](https://github.com/koment-dev/koment/commit/9fbd5821bb5a19a0d46dab66ec5efde0b2000de5))

## [0.1.1](https://github.com/koment-dev/koment/compare/v0.1.0...v0.1.1) (2026-08-02)


### Features

* add koment ui, a local read-only view of annotated code ([2b50b72](https://github.com/koment-dev/koment/commit/2b50b7231eec50bb7cb17781bee827c2d17ee702))
* add reanchor so drift can be fixed without hand-editing YAML ([94cc26b](https://github.com/koment-dev/koment/commit/94cc26b214b989f5b3e3a014ad70a7f52e9d627c))
* add the annotation store and anchor resolution ([39feef9](https://github.com/koment-dev/koment/commit/39feef9558ade67361ad94e65c1622aa99885578))
* add the CLI and the MCP server ([22f2e1b](https://github.com/koment-dev/koment/commit/22f2e1bece11167de385f7675d82b2047da92514))
* v2 foundations — provenance, toolchain, demo fixture ([#4](https://github.com/koment-dev/koment/issues/4)) ([f8995cf](https://github.com/koment-dev/koment/commit/f8995cfff2a5c44c1ebb45020652196a1e3cffc4))


### Documentation

* accept a local read-only web UI (ADR 0013) ([a305981](https://github.com/koment-dev/koment/commit/a305981bd0922ec27608f6566f5b57f58babca56))
* put users first, and add a demo site ([#3](https://github.com/koment-dev/koment/issues/3)) ([c5d12fb](https://github.com/koment-dev/koment/commit/c5d12fbd8f95821b46dda2a1cb0ffefbdf923b7c))
* record the design and the decisions behind it ([ebc28cf](https://github.com/koment-dev/koment/commit/ebc28cf59fbeda7c55ecec4d94ceae2c03d84ad4))
