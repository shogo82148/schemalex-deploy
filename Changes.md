# Changes

## [unreleased]

- support generated columns

## [v0.1.7] - 2026-09-23

- bump Go 1.27.1 [#263](https://github.com/shogo82148/schemalex-deploy/pull/263)

## [v0.1.6] - 2026-08-07

- support removing indexes without their names [#68](https://github.com/shogo82148/schemalex-deploy/pull/68)
- fix ignoring the white spaces before the equal sign in my.cnf [#221](https://github.com/shogo82148/schemalex-deploy/pull/221)

## [v0.1.5] - 2025-11-13

- support long SQL statement [#200](https://github.com/shogo82148/schemalex-deploy/pull/200)

## [v0.1.4] - 2025-10-21

- Fix: Support MySQL conditional comments in FULLTEXT INDEX with WITH PARSER [#193](https://github.com/shogo82148/schemalex-deploy/pull/193)
- bump Go 1.25.3 [#195](https://github.com/shogo82148/schemalex-deploy/pull/195)

## [v0.1.3] - 2024-08-01

## [v0.1.2] - 2024-04-11

- Resolved the issue of being unable to retrieve port numbers and usernames from configuration files [#115](https://github.com/shogo82148/schemalex-deploy/pull/115)
- bump Go 1.22 [#116](https://github.com/shogo82148/schemalex-deploy/pull/116)

## [v0.1.1] - 2023-12-22

- Fixed the issue where the port wasn't being read from my.cnf [#106](https://github.com/shogo82148/schemalex-deploy/pull/106)

## [v0.1.0] - 2023-11-08

- bump Go 1.21 [#101](https://github.com/shogo82148/schemalex-deploy/pull/101)

## [v0.0.9] - 2023-06-20

- fix parsing spatial index [#91](https://github.com/shogo82148/schemalex-deploy/pull/91)

## [v0.0.8] - 2023-06-05

- bump Go 1.20 [#87](https://github.com/shogo82148/schemalex-deploy/pull/87)
- support SRID [#86](https://github.com/shogo82148/schemalex-deploy/pull/86)

## [v0.0.7] - 2022-09-08

- support `--import` option [#60](https://github.com/shogo82148/schemalex-deploy/pull/60), [#61](https://github.com/shogo82148/schemalex-deploy/pull/61)
- bump Go 1.19 [#63](https://github.com/shogo82148/schemalex-deploy/pull/63)

## [v0.0.6] - 2022-07-15

- Support dry-run option [#57](https://github.com/shogo82148/schemalex-deploy/pull/57)

## [v0.0.5] - 2022-05-18

- arrange alter table statement [#53](https://github.com/shogo82148/schemalex-deploy/pull/53)
- bump Go 1.18 [#48](https://github.com/shogo82148/schemalex-deploy/pull/48)

## [v0.0.4] - 2021-09-02

- support unix domain socket [#32](https://github.com/shogo82148/schemalex-deploy/pull/32)
- refactor the parser [#31](https://github.com/shogo82148/schemalex-deploy/pull/31)
- publish to my rpm repository [#25](https://github.com/shogo82148/schemalex-deploy/pull/25)

## [v0.0.3] - 2021-08-15

- fix ~/.my.cnf is not loaded [#19](https://github.com/shogo82148/schemalex-deploy/pull/19)

## [v0.0.2] - 2021-08-15

- show the preview [#18](https://github.com/shogo82148/schemalex-deploy/pull/18)
- load the configure from files [#16](https://github.com/shogo82148/schemalex-deploy/pull/16), [#17](https://github.com/shogo82148/schemalex-deploy/pull/17)
- implement my.cnf parser [#15](https://github.com/shogo82148/schemalex-deploy/pull/15)
- remove reading stdin feature [#14](https://github.com/shogo82148/schemalex-deploy/pull/14)
- fix wrong version information [#13](https://github.com/shogo82148/schemalex-deploy/pull/13)

## [v0.0.1] - 2021-08-07

- initial release

[unreleased]: https://github.com/shogo82148/schemalex-deploy/compare/v0.1.7...main
[v0.1.7]: https://github.com/shogo82148/schemalex-deploy/compare/v0.1.6...v0.1.7
[v0.1.6]: https://github.com/shogo82148/schemalex-deploy/compare/v0.1.5...v0.1.6
[v0.1.5]: https://github.com/shogo82148/schemalex-deploy/compare/v0.1.4...v0.1.5
[v0.1.4]: https://github.com/shogo82148/schemalex-deploy/compare/v0.1.3...v0.1.4
[v0.1.3]: https://github.com/shogo82148/schemalex-deploy/compare/v0.1.2...v0.1.3
[v0.1.2]: https://github.com/shogo82148/schemalex-deploy/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/shogo82148/schemalex-deploy/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.9...v0.1.0
[v0.0.9]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.8...v0.0.9
[v0.0.8]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.7...v0.0.8
[v0.0.7]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.6...v0.0.7
[v0.0.6]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.5...v0.0.6
[v0.0.5]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.4...v0.0.5
[v0.0.4]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.3...v0.0.4
[v0.0.3]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.2...v0.0.3
[v0.0.2]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.1...v0.0.2
[v0.0.1]: https://github.com/shogo82148/schemalex-deploy/compare/v0.0.0...v0.0.1
