# Changelog

## [1.27.0](https://github.com/martinhg/capiko-ai/compare/v1.26.0...v1.27.0) (2026-08-22)


### Features

* **cmd:** add review mode CLI command and dispatch wiring (R-1 PR6/6) ([#220](https://github.com/martinhg/capiko-ai/issues/220)) ([2f8bc80](https://github.com/martinhg/capiko-ai/commit/2f8bc80dc9c6827dfc81e2c69cf844222915ea60))
* **cmd:** add review mode enable/disable/status CLI command ([48e43a4](https://github.com/martinhg/capiko-ai/commit/48e43a45544b5fdc9f70b664bdd3aad0152c884a))
* **cmd:** wire review command into dispatch and usage ([e009af7](https://github.com/martinhg/capiko-ai/commit/e009af7e535f7e109056668be13dfc51f766bb57))
* **rdd:** add 12-state lifecycle machine with transition table ([6206b58](https://github.com/martinhg/capiko-ai/commit/6206b58983091c5b69e7368406588bb6446610f1))
* **rdd:** add 12-state lifecycle machine with transition table (R-2 PR1/6) ([edc8c0d](https://github.com/martinhg/capiko-ai/commit/edc8c0de98ab469004de19d49128d096f7e1df35))
* **rdd:** add 4 canonical lens mandates with tier-based selection ([2778c80](https://github.com/martinhg/capiko-ai/commit/2778c80774af51b90d15b9799d9c4566956878f9))
* **rdd:** add CandidateIdentity and deterministic digest/compare ([2b75fb8](https://github.com/martinhg/capiko-ai/commit/2b75fb8884277f0821c804bbe4a81c8d3c2b7635))
* **rdd:** add CandidateIdentity and RiskClassifier (R-1 PR1/6) ([#215](https://github.com/martinhg/capiko-ai/issues/215)) ([640707b](https://github.com/martinhg/capiko-ai/commit/640707b48a5bb060ae76e29670b39fc196b19303))
* **rdd:** add contract-based consent type and flag parser ([a7642c2](https://github.com/martinhg/capiko-ai/commit/a7642c2a5f91be938da62d7830a0f1c828324339))
* **rdd:** add deterministic tier 0/1 risk classifier ([f3b2a89](https://github.com/martinhg/capiko-ai/commit/f3b2a89807e15a6e68aafb73dc6c2825a7137f19))
* **rdd:** add kill switch mode resolution ([9811bb8](https://github.com/martinhg/capiko-ai/commit/9811bb8cc4425308ff2e8eee124dfa875b77630e))
* **rdd:** add kill switch mode resolution and persistence (R-1 PR2/6) ([#216](https://github.com/martinhg/capiko-ai/issues/216)) ([9a3a013](https://github.com/martinhg/capiko-ai/commit/9a3a013adedb59335fc9303f977864fea6a29649))
* **rdd:** add tier 4 classification, lens mandates, and consent type (R-2 PR3/6) ([69e9199](https://github.com/martinhg/capiko-ai/commit/69e9199bb83f69de75b7811b914b336193cd9e25))
* **rdd:** add tier 4 risk classification with hot-path patterns ([46fb745](https://github.com/martinhg/capiko-ai/commit/46fb745d3bbb1f74ccdb904c5aa895d8fcceef1b))
* **rdd:** extend relation algebra to 7 values with ancestry evidence ([c265cea](https://github.com/martinhg/capiko-ai/commit/c265ceafa55c289b1c171105e31f778f7c30c328))
* **rdd:** extend relation algebra to 7 values with ancestry evidence (R-2 PR2/6) ([d54573d](https://github.com/martinhg/capiko-ai/commit/d54573d20daba41f89018b678ea59a264be5b26a))
* **release:** sign releases with cosign keyless ([50ddff4](https://github.com/martinhg/capiko-ai/commit/50ddff49b24f383f97860f08a8a3647af980b9cc))
* **release:** sign releases with cosign keyless ([#213](https://github.com/martinhg/capiko-ai/issues/213)) ([7b04c5b](https://github.com/martinhg/capiko-ai/commit/7b04c5be174be5055256e99764dc99b2f4fb2b2d))
* **reviewstore:** add CAS-protected authority store ([69c999c](https://github.com/martinhg/capiko-ai/commit/69c999c5dfd3b7009dc8e0f01c71b4138ff2d3c6))
* **reviewstore:** add CAS-protected authority store (R-1 PR5/6) ([#219](https://github.com/martinhg/capiko-ai/issues/219)) ([6db42fb](https://github.com/martinhg/capiko-ai/commit/6db42fb642661e2e61d91db10bde6b875783416d))
* **reviewstore:** add flock locking and git command seams (R-1 PR3/6) ([#217](https://github.com/martinhg/capiko-ai/issues/217)) ([d9ad311](https://github.com/martinhg/capiko-ai/commit/d9ad311b9c29cc70729835404047b8f85af51c8b))
* **reviewstore:** add flock-based single-writer locking ([e23f75f](https://github.com/martinhg/capiko-ai/commit/e23f75feb172df8d8b35a68cce900e89a7119c3c))
* **reviewstore:** add git command seams and output parsers ([5453df9](https://github.com/martinhg/capiko-ai/commit/5453df999b724e4600ba114d849c1dda89926cd6))
* **reviewstore:** add kill switch mode persistence ([809c121](https://github.com/martinhg/capiko-ai/commit/809c1214d5dd4fb1dcefbf8682a3ce7a94b05fee))
* **reviewstore:** orchestrate git seams into BuildIdentity ([bf9c18a](https://github.com/martinhg/capiko-ai/commit/bf9c18aa4724806ec2a7cd4d2b2e5287d43e9412))
* **reviewstore:** orchestrate git seams into BuildIdentity (R-1 PR4/6) ([#218](https://github.com/martinhg/capiko-ai/issues/218)) ([6343918](https://github.com/martinhg/capiko-ai/commit/634391890bd934d0039318eb99ea83c301961a04))

## [1.26.0](https://github.com/martinhg/capiko-ai/compare/v1.25.0...v1.26.0) (2026-08-20)


### Features

* **tui:** migrate to bubbletea v2 + lipgloss v2, add session probe hook, bump versions ([36a8a9b](https://github.com/martinhg/capiko-ai/commit/36a8a9becefc8e23ec5a53a3ebd7d48fdf0c6127))

## [1.25.0](https://github.com/martinhg/capiko-ai/compare/v1.24.0...v1.25.0) (2026-08-20)


### Features

* **cga:** add applyCGA/cleanupGGA backend, remove GGA ([#193](https://github.com/martinhg/capiko-ai/issues/193)) ([84ae30c](https://github.com/martinhg/capiko-ai/commit/84ae30cc5c70ac806236acdca282fc25c8f6c45b))
* **cga:** add cga learn/rules CLI and JSON persistence (Phase 3 PR3) ([#204](https://github.com/martinhg/capiko-ai/issues/204)) ([2ddab79](https://github.com/martinhg/capiko-ai/commit/2ddab79207a5d4aae4996802e41f99d1a598e89c))
* **cga:** add findings CLI subcommand (Phase 2 PR4) ([#201](https://github.com/martinhg/capiko-ai/issues/201)) ([714b66e](https://github.com/martinhg/capiko-ai/commit/714b66edca883e4bb380a561cb07783e8354f98d))
* **cga:** add findings log parser (Phase 2 PR1) ([#198](https://github.com/martinhg/capiko-ai/issues/198)) ([8b05c3b](https://github.com/martinhg/capiko-ai/commit/8b05c3bef0d2b900d3cfce043238899dda414cc1))
* **cga:** add findings-log append and post-commit SHA patch (Phase 2 PR2) ([#199](https://github.com/martinhg/capiko-ai/issues/199)) ([1d65929](https://github.com/martinhg/capiko-ai/commit/1d659291077a9e121444dec95349770c63d09c30))
* **cga:** add pattern detection for learn-loop (Phase 3 PR1) ([#202](https://github.com/martinhg/capiko-ai/issues/202)) ([6e91197](https://github.com/martinhg/capiko-ai/commit/6e91197fd59f99ef0f3897837731ff6a562b253a))
* **cga:** add pure CGA package (prompt, verdict, hook) ([#190](https://github.com/martinhg/capiko-ai/issues/190)) ([7c4d9ea](https://github.com/martinhg/capiko-ai/commit/7c4d9eabce275f813059d6038e0528ca135fd59e))
* **cga:** add rule composition and bounded growth (Phase 3 PR2) ([#203](https://github.com/martinhg/capiko-ai/issues/203)) ([f8dde6b](https://github.com/martinhg/capiko-ai/commit/f8dde6bd7fdccef262eedfb936add57b96eba05c))
* **cga:** add scope discipline to learned rules (Phase 4) ([#206](https://github.com/martinhg/capiko-ai/issues/206)) ([377581b](https://github.com/martinhg/capiko-ai/commit/377581b82be34d88eb5d6bc6ccae827c9d5bd245))
* **cga:** re-apply pre-commit hook in RunSync (CGA Phase 0 PR5) ([#195](https://github.com/martinhg/capiko-ai/issues/195)) ([6d3ec70](https://github.com/martinhg/capiko-ai/commit/6d3ec70c203ad5d57e3ad440a796c924bb32dd78))
* **cga:** rename screen to cgaScreen, add Timeout row ([#194](https://github.com/martinhg/capiko-ai/issues/194)) ([fde62f8](https://github.com/martinhg/capiko-ai/commit/fde62f83b763e4dba1877303196533b85c3fcb9e))
* **cga:** structured verdict with per-file findings (CGA Phase 1) ([#197](https://github.com/martinhg/capiko-ai/issues/197)) ([9589454](https://github.com/martinhg/capiko-ai/commit/95894543ea061e6468a7a183e0c2b261c270b0ba))
* **cga:** wire post-commit hook lifecycle (Phase 2 PR3) ([#200](https://github.com/martinhg/capiko-ai/issues/200)) ([d75725a](https://github.com/martinhg/capiko-ai/commit/d75725addcb59fce4951ed2849d955bb7d5e10db))
* **state:** add CGARecord and SetCGA ([#192](https://github.com/martinhg/capiko-ai/issues/192)) ([90bd540](https://github.com/martinhg/capiko-ai/commit/90bd540d277d92d261053f4905138974916f0970))


### Bug Fixes

* **cga:** correct engram save CLI flags and add scope filter ([#207](https://github.com/martinhg/capiko-ai/issues/207)) ([671af47](https://github.com/martinhg/capiko-ai/commit/671af4703a293d6eed16fb4ee195c5c295e68aa6))
* **cga:** harden GGA cleanup, add recursion guard, drop GGA from sysinfo ([#196](https://github.com/martinhg/capiko-ai/issues/196)) ([a857705](https://github.com/martinhg/capiko-ai/commit/a8577057a69c49354d3cc6de082dde6458b6e3e6))

## [1.24.0](https://github.com/martinhg/capiko-ai/compare/v1.23.0...v1.24.0) (2026-08-13)


### Features

* **tui:** seed SDD orchestrator block on headless install and sync (K-18) ([#180](https://github.com/martinhg/capiko-ai/issues/180)) ([e064e2f](https://github.com/martinhg/capiko-ai/commit/e064e2feabc1e3363f9109bd29cdb17dce887f76))
* **tui:** show contextual description for focused menu item ([#185](https://github.com/martinhg/capiko-ai/issues/185)) ([70d32d3](https://github.com/martinhg/capiko-ai/commit/70d32d336b460c455d8916bf79b7717d10840152))


### Bug Fixes

* **sdd:** use ambiguity heuristic for SDD triage gate, add user override (K-14) ([#187](https://github.com/martinhg/capiko-ai/issues/187)) ([14332d3](https://github.com/martinhg/capiko-ai/commit/14332d32738f24a73073b80e872728174555e22d))
* **skills:** harden SDD phase skills against phantom files, UUID false positives, and missing dirs ([#181](https://github.com/martinhg/capiko-ai/issues/181)) ([eaf3142](https://github.com/martinhg/capiko-ai/commit/eaf3142bda75791a644c491c2605cabcfc7a2369))

## [1.23.0](https://github.com/martinhg/capiko-ai/compare/v1.22.0...v1.23.0) (2026-08-06)


### Features

* **agent:** per-phase model routing via .agent.md frontmatter ([#164](https://github.com/martinhg/capiko-ai/issues/164)) ([0f01faf](https://github.com/martinhg/capiko-ai/commit/0f01faf6d68cf60524adde58e71e096e8c62d61e))
* **backup:** validate restore targets against allowed roots (F-S3) ([#173](https://github.com/martinhg/capiko-ai/issues/173)) ([1ee6ce0](https://github.com/martinhg/capiko-ai/commit/1ee6ce091c94b63f005cf46b891d8455a576c4da))
* **catalog:** interactive proposal questions + judgment-day agents (K-18) ([#179](https://github.com/martinhg/capiko-ai/issues/179)) ([e027226](https://github.com/martinhg/capiko-ai/commit/e0272269c6dd74371dc823bfecfa0ffecebf5984))
* **catalog:** prompt-injection defence clause in file-reading skills (F-AI3) ([#175](https://github.com/martinhg/capiko-ai/issues/175)) ([4d3eb97](https://github.com/martinhg/capiko-ai/commit/4d3eb97bb8ecf2a25e010f51d5d5d2e1c65f917a))
* **cli:** headless backup list and restore subcommands (G-CLI3) ([#176](https://github.com/martinhg/capiko-ai/issues/176)) ([e74dcf7](https://github.com/martinhg/capiko-ai/commit/e74dcf7fc92513c85b49f160e1d1fca60c86af23))
* **cli:** help, unknown-command errors, and non-TTY guard (G-CLI1) ([#170](https://github.com/martinhg/capiko-ai/issues/170)) ([d30127b](https://github.com/martinhg/capiko-ai/commit/d30127bff602340b65c137c507bf3fca57777684))
* **contract:** Copilot CLI schema contract tests (G-CC4) ([#172](https://github.com/martinhg/capiko-ai/issues/172)) ([44618e3](https://github.com/martinhg/capiko-ai/commit/44618e3adbe000dd376b53d2387ffba897ae0f57))
* **copilot:** add copilot-managed-hooks foundation ($COPILOT_HOME + state record) ([#152](https://github.com/martinhg/capiko-ai/issues/152)) ([a08ea89](https://github.com/martinhg/capiko-ai/commit/a08ea893220b419508f16c459cc6095c4de2d462))
* **copilothooks:** add atomic hook-file writer and combined checksum ([#155](https://github.com/martinhg/capiko-ai/issues/155)) ([ac394de](https://github.com/martinhg/capiko-ai/commit/ac394de8dd6fff2c1a12528894b1fee465ad71a0))
* **copilothooks:** add sessionStart verification hook and doctor check (G-CC2) ([#165](https://github.com/martinhg/capiko-ai/issues/165)) ([ce2a958](https://github.com/martinhg/capiko-ai/commit/ce2a958580a3f9b0ff0b60193bc9421379f7c280))
* **copilothooks:** add v1 hook schema and guardrails renderer ([#154](https://github.com/martinhg/capiko-ai/issues/154)) ([7e75536](https://github.com/martinhg/capiko-ai/commit/7e755362ffe3c49a5c8f07b12fc48b3bf05a847f))
* **doctor:** Copilot CLI version-skew check (G-CC1) ([#171](https://github.com/martinhg/capiko-ai/issues/171)) ([e4942c1](https://github.com/martinhg/capiko-ai/commit/e4942c102b59d8c6ed78ea3c610e72072933e86e))
* **doctor:** instruction budget check with per-block breakdown (F-AI2) ([#174](https://github.com/martinhg/capiko-ai/issues/174)) ([d1b2b3d](https://github.com/martinhg/capiko-ai/commit/d1b2b3d8f0469c2deebbc1f02cbc7b826820c95b))
* **engram:** Key Learnings in sub-agents + lifecycle dedup (K-10, K-11) ([#166](https://github.com/martinhg/capiko-ai/issues/166)) ([03a112d](https://github.com/martinhg/capiko-ai/commit/03a112dcd4b95b26801775952491ddd98a3d0a1f))
* **integrity:** SHA-256 protected-surface integrity manifest (K-2) ([#169](https://github.com/martinhg/capiko-ai/issues/169)) ([33f3d8e](https://github.com/martinhg/capiko-ai/commit/33f3d8ea5af66cf4b000715ac290cf4af6bbe50b))
* **sdd:** model fallback/rotation on token/quota exhaustion (K-1) ([#168](https://github.com/martinhg/capiko-ai/issues/168)) ([246e1ed](https://github.com/martinhg/capiko-ai/commit/246e1edaf827324afe4e02e910bf391b5d6cf511))
* **skillregistry:** content-hash fingerprint for staleness detection (K-13) ([#167](https://github.com/martinhg/capiko-ai/issues/167)) ([47b0ae6](https://github.com/martinhg/capiko-ai/commit/47b0ae69b9741010204280ec050fe03958eee38e))
* **tui:** add copilot-hooks apply/disable orchestration, drift, and RunSync gate ([#156](https://github.com/martinhg/capiko-ai/issues/156)) ([12e1231](https://github.com/martinhg/capiko-ai/commit/12e1231ea5bf977b29c74682f26db19011835a4c))
* **tui:** Copilot hooks configure screen + menu wiring (PR-5) ([#157](https://github.com/martinhg/capiko-ai/issues/157)) ([622ef31](https://github.com/martinhg/capiko-ai/commit/622ef310cdd990d9fafb555b774b5d2589c8f4f6))

## [1.22.0](https://github.com/martinhg/capiko-ai/compare/v1.21.0...v1.22.0) (2026-06-29)


### Features

* **githooks:** marker-delimited git hook block writer/remover ([#146](https://github.com/martinhg/capiko-ai/issues/146)) ([a9beb9e](https://github.com/martinhg/capiko-ai/commit/a9beb9ee4a2786632ebce198274975a53ffc7c3e))
* **teamsync:** Configure team sync TUI screen + menu + docs ([#149](https://github.com/martinhg/capiko-ai/issues/149)) ([71d5472](https://github.com/martinhg/capiko-ai/commit/71d54723cad9e082e69f9d0cc7c00238dec8ea4f))
* **teamsync:** engram project resolver + hook conflict detection + body builders ([#147](https://github.com/martinhg/capiko-ai/issues/147)) ([1d11776](https://github.com/martinhg/capiko-ai/commit/1d11776ce92f11f6a9d1cd3ca511c3d858dbabd0))
* **teamsync:** state record + apply/disable hook orchestration ([#148](https://github.com/martinhg/capiko-ai/issues/148)) ([774a626](https://github.com/martinhg/capiko-ai/commit/774a626db4069e7dc6855ea7156aaf144cdc4e5e))

## [1.21.0](https://github.com/martinhg/capiko-ai/compare/v1.20.0...v1.21.0) (2026-06-29)


### Features

* **sddstatus:** resolveEngramStatus fallback + Resolve wiring (3/3) ([#141](https://github.com/martinhg/capiko-ai/issues/141)) ([80b2041](https://github.com/martinhg/capiko-ai/commit/80b20412fe906affcd8436bfaac34b189fa662b3))

## [1.20.0](https://github.com/martinhg/capiko-ai/compare/v1.19.0...v1.20.0) (2026-06-29)


### Features

* **memory:** add engram proactive-memory protocol block ([#137](https://github.com/martinhg/capiko-ai/issues/137)) ([4ee45ed](https://github.com/martinhg/capiko-ai/commit/4ee45ed71fe435b8f5f813985a64576550fdc486))
* **sddstatus:** Engram observation seam and artifact helpers (2/3) ([#140](https://github.com/martinhg/capiko-ai/issues/140)) ([d3ccd8d](https://github.com/martinhg/capiko-ai/commit/d3ccd8d0bf279950d8d9e63931de651ac4193d21))

## [1.19.0](https://github.com/martinhg/capiko-ai/compare/v1.18.0...v1.19.0) (2026-06-21)


### Features

* **backup:** symmetric agent backup before destructive ops ([#133](https://github.com/martinhg/capiko-ai/issues/133)) ([2b511a7](https://github.com/martinhg/capiko-ai/commit/2b511a732bb6275040210879dcd8a98499d79a97))

## [1.18.0](https://github.com/martinhg/capiko-ai/compare/v1.17.0...v1.18.0) (2026-06-21)


### Features

* surface gga in detection and offer code-review after install ([#131](https://github.com/martinhg/capiko-ai/issues/131)) ([5d9544d](https://github.com/martinhg/capiko-ai/commit/5d9544d32f705fe5b7f8899c7b3a1c80301c8ce6))

## [1.17.0](https://github.com/martinhg/capiko-ai/compare/v1.16.0...v1.17.0) (2026-06-21)


### Features

* **cli:** structured --verbose logging for CLI subcommands ([#129](https://github.com/martinhg/capiko-ai/issues/129)) ([d2a948d](https://github.com/martinhg/capiko-ai/commit/d2a948d9abb44517c5c8866199497293c5f70a39))

## [1.16.0](https://github.com/martinhg/capiko-ai/compare/v1.15.0...v1.16.0) (2026-06-21)


### Features

* **skill:** enforce a Trigger clause in skill descriptions ([#127](https://github.com/martinhg/capiko-ai/issues/127)) ([81a66da](https://github.com/martinhg/capiko-ai/commit/81a66da0713dea813374b68dcea07bc863338c1d))

## [1.15.0](https://github.com/martinhg/capiko-ai/compare/v1.14.0...v1.15.0) (2026-06-21)


### Features

* **tui:** configure Gentleman Guardian Angel (gga) code review ([#125](https://github.com/martinhg/capiko-ai/issues/125)) ([4cba132](https://github.com/martinhg/capiko-ai/commit/4cba132fde62ef211d665658183bc136eeade927))

## [1.14.0](https://github.com/martinhg/capiko-ai/compare/v1.13.0...v1.14.0) (2026-06-21)


### Features

* **tui:** add SDD Status dashboard screen ([#123](https://github.com/martinhg/capiko-ai/issues/123)) ([c258319](https://github.com/martinhg/capiko-ai/commit/c258319401512caa4ba70a7a59a26caf4120e01b))

## [1.13.0](https://github.com/martinhg/capiko-ai/compare/v1.12.0...v1.13.0) (2026-06-21)


### Features

* **skill:** dependency graph with validation, auto-install, and doctor reporting ([#120](https://github.com/martinhg/capiko-ai/issues/120)) ([d6da05b](https://github.com/martinhg/capiko-ai/commit/d6da05be51e9d7c4b3888981502c04af201cc053))

## [1.12.0](https://github.com/martinhg/capiko-ai/compare/v1.11.0...v1.12.0) (2026-06-21)


### Features

* **headroom:** verify MCP command, add drift detection, ship NOTICE ([#117](https://github.com/martinhg/capiko-ai/issues/117)) ([4936477](https://github.com/martinhg/capiko-ai/commit/49364778bacf73917ff88433badc6a3481df8f7e))

## [1.11.0](https://github.com/martinhg/capiko-ai/compare/v1.10.0...v1.11.0) (2026-06-21)


### Features

* **headroom:** inject agent guidance to use the compression tools ([#115](https://github.com/martinhg/capiko-ai/issues/115)) ([a844b0c](https://github.com/martinhg/capiko-ai/commit/a844b0cd571457fcfb6de0210ba82160314a3938))

## [1.10.0](https://github.com/martinhg/capiko-ai/compare/v1.9.0...v1.10.0) (2026-06-21)


### Features

* **headroom:** wire headroom context-compression via MCP, with a Configure screen ([#113](https://github.com/martinhg/capiko-ai/issues/113)) ([c9dcf3d](https://github.com/martinhg/capiko-ai/commit/c9dcf3d1f1f61089bd25f985d274d1e2233f1e6e))

## [1.9.0](https://github.com/martinhg/capiko-ai/compare/v1.8.0...v1.9.0) (2026-06-20)


### Features

* **efficiency:** add opt-in output-efficiency instruction block ([#111](https://github.com/martinhg/capiko-ai/issues/111)) ([22e216a](https://github.com/martinhg/capiko-ai/commit/22e216a0891fd6daf91a25a52ca43790dd368e7c))

## [1.8.0](https://github.com/martinhg/capiko-ai/compare/v1.7.0...v1.8.0) (2026-06-20)


### Features

* **doctor:** add --repair to re-apply the managed catalog on drift ([#108](https://github.com/martinhg/capiko-ai/issues/108)) ([2cd6262](https://github.com/martinhg/capiko-ai/commit/2cd6262e105b135552e2b334bd24823ff8b8f764))
* **doctor:** report last update-check time ([#105](https://github.com/martinhg/capiko-ai/issues/105)) ([3fcfcea](https://github.com/martinhg/capiko-ai/commit/3fcfcea34a95da73ad82c24e8fbb558fe063bb5a))
* **engram:** warn when a managed engram binary is outdated ([#109](https://github.com/martinhg/capiko-ai/issues/109)) ([9a24ddd](https://github.com/martinhg/capiko-ai/commit/9a24ddd3f6dd2698797cebf44fb29b112ebe7938))
