# Changelog

## [0.5.1](https://github.com/janpuc/koment/compare/v0.5.0...v0.5.1) (2026-08-05)


### Bug Fixes

* stop the install checks racing the release they verify ([#33](https://github.com/janpuc/koment/issues/33)) ([54d9f40](https://github.com/janpuc/koment/commit/54d9f40994b5407c07d7511bd20fc3e25b64d475))

## [0.5.0](https://github.com/janpuc/koment/compare/v0.4.0...v0.5.0) (2026-08-05)


### Features

* give an annotation a title so nothing is shown cut off ([#31](https://github.com/janpuc/koment/issues/31)) ([ce4725a](https://github.com/janpuc/koment/commit/ce4725a0f199e7165753771470171af0a16e43a8))

## [0.4.0](https://github.com/janpuc/koment/compare/v0.3.1...v0.4.0) (2026-08-04)


### Features

* give rationale a panel and stop marking healthy code ([#30](https://github.com/janpuc/koment/issues/30)) ([e154ed6](https://github.com/janpuc/koment/commit/e154ed6dd9078df0e266146a78c5536fe52a71d1))
* make the editor and the toolchain find koment ([#27](https://github.com/janpuc/koment/issues/27)) ([e144e61](https://github.com/janpuc/koment/commit/e144e614b44fe4848b0d29dc3aa1b0225e0c632c))


### Bug Fixes

* stop the language server putting null where LSP declares an array ([#28](https://github.com/janpuc/koment/issues/28)) ([8b78d72](https://github.com/janpuc/koment/commit/8b78d721a878a7b3d328fb18c334cd0362398b81))

## [0.3.1](https://github.com/janpuc/koment/compare/v0.3.0...v0.3.1) (2026-08-04)


### Bug Fixes

* repair the three publish failures that emptied 0.3.0 ([#25](https://github.com/janpuc/koment/issues/25)) ([4a4f530](https://github.com/janpuc/koment/commit/4a4f530a7983cdd1c09c0403911f8a029cff0ebc))

## [0.3.0](https://github.com/janpuc/koment/compare/v0.2.0...v0.3.0) (2026-08-04)


### Features

* add local writes and policy enforcement ([#20](https://github.com/janpuc/koment/issues/20)) ([2829410](https://github.com/janpuc/koment/commit/2829410405f9e1bbbdb797079601ab2ef24bcf9d))
* distribute koment through package managers, marketplaces and editors ([#21](https://github.com/janpuc/koment/issues/21)) ([9d5f421](https://github.com/janpuc/koment/commit/9d5f421337f9681286f43376d418a7732a204542))
* reset annotation records and anchors ([#19](https://github.com/janpuc/koment/issues/19)) ([2084861](https://github.com/janpuc/koment/commit/208486110e29da25e515684b3917ff94ce819bbf))


### Bug Fixes

* keep the chart README generatable across a version bump ([#22](https://github.com/janpuc/koment/issues/22)) ([4139feb](https://github.com/janpuc/koment/commit/4139feb2ae984c71422c4a4dc3ec18fe3a271b3d))
* stop deleting the helm test pod before its logs are read ([#23](https://github.com/janpuc/koment/issues/23)) ([2ff1ed4](https://github.com/janpuc/koment/commit/2ff1ed471065e23cd09057197ed8d26400b76168))


### Documentation

* reset architecture for vNext ([#17](https://github.com/janpuc/koment/issues/17)) ([1d12296](https://github.com/janpuc/koment/commit/1d122963f024f48ed04833c6148369f92bbdebbc))


### Continuous Integration

* exercise the setup action against a published release ([#15](https://github.com/janpuc/koment/issues/15)) ([c5ad926](https://github.com/janpuc/koment/commit/c5ad926365542b1db0ebec343978d5b9a5b3e21a))
* name every job after its id and stop paying for setup twice ([#24](https://github.com/janpuc/koment/issues/24)) ([963b574](https://github.com/janpuc/koment/commit/963b57417318160647448f2b81712db83ea5dcde))

## [0.2.0](https://github.com/janpuc/koment/compare/v0.1.2...v0.2.0) (2026-08-03)


### Features

* .koment bootstraps the index, and export rebuilds .koment from it ([#9](https://github.com/janpuc/koment/issues/9)) ([90357ea](https://github.com/janpuc/koment/commit/90357ea3b697f1235c7937484010cf43dd1654ad))
* a nested file tree, a repository switcher, and notes that float ([#13](https://github.com/janpuc/koment/issues/13)) ([725f526](https://github.com/janpuc/koment/commit/725f526da5dbc756cae655edcdfb96f5650963cd))
* multi-repository is first class, not a second citizen ([#11](https://github.com/janpuc/koment/issues/11)) ([5655d1b](https://github.com/janpuc/koment/commit/5655d1b23479e361e9cc9d0358f0d0b0cbfb0249))
* publishing is a first-class tier, not demo scaffolding ([#12](https://github.com/janpuc/koment/issues/12)) ([7da5802](https://github.com/janpuc/koment/commit/7da5802d868b3cefd37043ef7f38b53095841088))

## [0.1.2](https://github.com/janpuc/koment/compare/v0.1.1...v0.1.2) (2026-08-02)


### Features

* index annotations in a database; git keeps the record ([#8](https://github.com/janpuc/koment/issues/8)) ([a28ac9d](https://github.com/janpuc/koment/commit/a28ac9dc89791a5d2a493f0fd858c933da128e7e))
* metrics, env configuration, and a two-repository demo ([#6](https://github.com/janpuc/koment/issues/6)) ([9fbd582](https://github.com/janpuc/koment/commit/9fbd5821bb5a19a0d46dab66ec5efde0b2000de5))

## [0.1.1](https://github.com/janpuc/koment/compare/v0.1.0...v0.1.1) (2026-08-02)


### Features

* add koment ui, a local read-only view of annotated code ([2b50b72](https://github.com/janpuc/koment/commit/2b50b7231eec50bb7cb17781bee827c2d17ee702))
* add reanchor so drift can be fixed without hand-editing YAML ([94cc26b](https://github.com/janpuc/koment/commit/94cc26b214b989f5b3e3a014ad70a7f52e9d627c))
* add the annotation store and anchor resolution ([39feef9](https://github.com/janpuc/koment/commit/39feef9558ade67361ad94e65c1622aa99885578))
* add the CLI and the MCP server ([22f2e1b](https://github.com/janpuc/koment/commit/22f2e1bece11167de385f7675d82b2047da92514))
* v2 foundations — provenance, toolchain, demo fixture ([#4](https://github.com/janpuc/koment/issues/4)) ([f8995cf](https://github.com/janpuc/koment/commit/f8995cfff2a5c44c1ebb45020652196a1e3cffc4))


### Documentation

* accept a local read-only web UI (ADR 0013) ([a305981](https://github.com/janpuc/koment/commit/a305981bd0922ec27608f6566f5b57f58babca56))
* put users first, and add a demo site ([#3](https://github.com/janpuc/koment/issues/3)) ([c5d12fb](https://github.com/janpuc/koment/commit/c5d12fbd8f95821b46dda2a1cb0ffefbdf923b7c))
* record the design and the decisions behind it ([ebc28cf](https://github.com/janpuc/koment/commit/ebc28cf59fbeda7c55ecec4d94ceae2c03d84ad4))
