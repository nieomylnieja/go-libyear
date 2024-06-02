# go-libyear

Calculate Go module's libyear!

## Install

Use pre-built binaries from
the [latest release](https://github.com/nieomylnieja/go-libyear/releases/latest)
or install
with Go:

```shell
go install github.com/nieomylnieja/go-libyear/cmd/go-libyear@latest
```

It can also be built directly from this repository:

```shell
git clone https://github.com/nieomylnieja/go-libyear.git
cd go-libyear
make build
./bin/go-libyear ./go.mod
```

### Docker

Docker images hosted on [GitHub Container Registry](https://github.com/nieomylnieja/go-libyear/pkgs/container/go-libyear)
are also provided:

```shell
docker pull ghcr.io/nieomylnieja/go-libyear:latest
docker run --rm ghcr.io/nieomylnieja/go-libyear -p github.com/nieomylnieja/go-libyear
```

## Usage

`go-libyear` can be used both as a CLI and Go library.
The CLI usage is also documented in [usage.txt](./cmd/go-libyear/usage.txt)
and accessed through `go-libyear --help`.

Basic usage:

```shell
$ go-libyear /path/to/go.mod
package                             version  date        latest   latest_date  libyear
github.com/nieomylnieja/go-libyear           2023-11-06                        2.41
github.com/pkg/errors               v0.8.1   2019-01-03  v0.9.1   2020-01-14   1.03
github.com/urfave/cli/v2            v2.20.0  2022-10-14  v2.25.7  2023-06-14   0.67
golang.org/x/mod                    v0.12.0  2023-06-21  v0.14.0  2023-10-25   0.35
golang.org/x/sync                   v0.3.0   2023-06-01  v0.5.0   2023-10-11   0.36
```

## Calculated metrics

### Libyear

What exactly is _libyear_?
Quoting and paraphrasing [libyear.com](https://libyear.com/):
> Libyear is a simple measure of software **dependency freshness**. \
> It is a single number telling you how **up-to-date** your dependencies
> are. \
> Example: pkg/errors _v0.8.1_ (June 2019) is **1 libyear** behind _v0.9.0_
> (June 2020).

Libyear is the default metric calculated by the program.

Example:

| Current | Current release | Latest | Latest release | Libyear |
|---------|-----------------|--------|----------------|---------|
| v1.45.1 | 2022-10-11      | v2.0.5 | 2023-10-11     | 1       |
| v1.46.0 | 2022-12-04      | v2.0.5 | 2023-10-11     | 0.85    |
| v2.0.0  | 2023-10-01      | v2.0.5 | 2022-10-11     | 0.03    |

### Number of releases

Dependencies with short release cycles are penalized by this measurement,
as the version sequence distance is relatively high compared to other
dependencies.

Example:

| Versions |
|----------|
| v1.45.1  |
| v1.45.2  |
| v1.46.0  |
| v2.0.0   |
| v2.0.1   |

| Current | Latest | Delta |
|---------|--------|-------|
| v1.45.1 | v2.0.5 | 5     |

### Version number delta

Version delta is a tuple (x,y,z) where:

- x is major version
- y is minor version
- z is patch version

Only highest-order version number is taken into consideration.

Example:

| Current | Latest  | Delta   |
|---------|---------|---------|
| v1.45.1 | v2.0.5  | (1,0,0) |
| v1.45.1 | v1.47.5 | (0,2,0) |
| v1.45.1 | v.45.5  | (0,0,4) |

### Results manipulation

<!-- markdownlint-disable MD013 -->

| Flag                  | Explanation                                                  |
|-----------------------|--------------------------------------------------------------|
| `--releases`          | Count number of releases between current and latest.         |
| `--versions`          | Calculate version number delta between current and latest.   |
| `--indirect`          | Include indirect dependencies in the results.                |
| `--skip-fresh`        | Skip up-to-date dependencies from the results.               |
| `--find-latest-major` | Use next, greater than or equal to v2 version as the latest. |

### Module sources

| Source      | Flag      | Example                                                                                                         |
|-------------|-----------|-----------------------------------------------------------------------------------------------------------------|
| File path   | _default_ | ~/my-project/go.mod                                                                                             |
| URL         | `--url`   | https://raw.githubusercontent.com/nieomylnieja/go-libyear/main/go.mod  <!-- markdownlint-disable-line MD034 --> |
| Module path | `--pkg`   | github.com/nieomylnieja/go-libyear@latest                                                                       |

<!-- markdownlint-enable MD013 -->

### Output formats

| Format | Flag      |
|--------|-----------|
| Table  | _default_ |
| JSON   | `--json`  |
| CSV    | `--csv`   |

### Historical data

In order to calculate the metrics in a given point in time,
use `--age-limit` flag.
Example:

```sh
go-libyear --age-limit 2022-10-01T12:00:00Z ./go.mod
```

The latest version for each package will be appointed as the latest version
of the package before or at the provided timestamp.

The flag works any other flag. If using a script to extract a history
of the calculated metrics, it is recommended to use `--cache` flag as well.

### Caching

`go-libyear` ships with a built-in caching mechanism.
It is disabled by default but can be enabled and adjusted with the following
flags:

| Flag                | Explanation                            |
|---------------------|----------------------------------------|
| `--cache`           | Enable caching.                        |
| `--cache-file-path` | Use the specified file for caching.    |
| `--vcs-cache-dir`   | Use custom cache path for VCS modules. |

## Go versioning

By default `go-libyear` will fetch the latest version for the current major
version adhering to the following rules:

- If the current major version is equal to 0.x.x and there's version 1.x.x
  available, set the latest to 1.x.x.
- If the current major is equal to or greater than 1.x.x, set the latest
  to 1.x.x.

If you wish to always set the next major version as the latest, you can use
the `--find-latest-major` (short `-M`) flag.
This flag enforces the following rules:

- If the current major is equal to or greater than x.x.x, set the latest
  to the latest (by semver) available version.
- If the latest major version is greater than the current major and the
  current version has been published after the first version of the latest
  major, the [libyear](#libyear) is calculated as a difference between the
  first version of the latest major and latest major version.

  Example:
  > Current version is 1.21.9 (2024-02-01), latest is 2.0.5 (2024-01-19); \
  1.21.9 was a security fix, it still means we're some time behind v2; \
  2.0.0 was released on 2024-01-02, this means we're 17 days
  (2024-01-19 - 2024-01-02) behind the latest v2, despite the fact that we've
  updated to the latest security patch for v1.

If you wish to not compensate for such cases and leave libyear as is
(it won't be ever negative, we round it to 0 if negative), use the
`--no-libyear-compensation` flag.

The [modules reference](https://go.dev/ref/mod) states that:
> If the module is released at major version 2 or higher,
  the module path must end with a major version suffix like /v2.

This is however not always the case, some older projects, usually pre-module,
might not adhere to that.
The aforementioned flag also works with such scenarios.

## Caveats

### Accessing private repositories

Currently the default mode of execution only supports git VCS and GitHub source.
To access all private modules use `--go-list` flag.
It will instruct the program to utilize `go list` command instead of GOPROXY API.

### Using `--go-list` flag

If `--go-list` flag is provided, `go-libyear` will used `go list` command to
fetch information about modules.
Specifically it runs `go list -m -mod=readonly`.
If the program is executed in a project containing a `go.mod` which `go.sum`
file is out of sync,
it will drop the following error:

```text
updates to go.sum needed, disabled by -mod=readonly
```

Due to that it is advised to stick with default modules information
provider.

## Development

CLI application is tested
with [bats framework](https://github.com/bats-core/bats-core).
The tests are defined in `test` folder.
Only core calculations are covered by unit tests, main paths are tested through
CLI tests.

## Acknowledgements

Inspired directly
by [SE Radio episode 587](https://www.se-radio.net/2023/10/se-radio-587-m-scott-ford-on-managing-dependency-freshness/).
Further reading through [libyear.com](https://libyear.com/) and
mimicking [libyear-bundler](https://github.com/jaredbeck/libyear-bundler) capabilities.

All the concepts and theory is based
on or directly quoted
from [Measuring Dependency Freshness in Software Systems][1].

[1]: <https://ericbouwers.github.io/papers/icse15.pdf> (J. Cox, E. Bouwers, M. van Eekelen and J. Visser, Measuring Dependency Freshness in Software Systems. In Proceedings of the 37th International Conference on Software Engineering, May 2015)
