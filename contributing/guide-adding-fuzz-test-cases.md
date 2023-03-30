# Fuzz testing

As a result of a [Security Audit][fuzz-audit-report] by Ada Logics sponsored by
the CNCF on the Crossplane project, we have hooked up both
`crossplane/crossplane` and `crossplane/crossplane-runtime` to be fuzz tested
by [OSS-Fuzz][oss-fuzz].

## TL;DR: How to add a new fuzz test

In order to add a fuzzer to any of the two repositories you'll just need to add
a new `Fuzz*` test case wherever makes more sense in one of the two
repositories, taking care to respect the assumptions defined in each
repository's `test/fuzz/oss_fuzz_build.sh` script,
[crossplane/crossplane][xp-fuzz_oss_build] and
[crossplane/crossplane-runtime][xp-r-fuzz_oss_build]. See [existing test
cases][xp-fuzz-tests] for examples.

## Fuzz testing

Fuzz testing is a way of testing programs by passing pseudo-random data to a
target function with the goal of finding bugs and vulnerabilities. Fuzz testing
is a form of dynamic analysis, which means that the program is executed with
the fuzzed data, and the execution is monitored for crashes and hangs.

Since version 1.18, Go has added native support for fuzz testing. This means
that alongside the usual `Test*` functions that are run by `go test`, we can
also define `Fuzz*` functions that will be run by `go test -fuzz
<FuzzTestName>`.

```go
package somepackage

import "testing"

func FuzzTestName(f *testing.F) {
	f.Fuzz(func(t *testing.T/*, ... some input we want to be randomized ...*/) {
		// ... use the randomized input to test the function ...
	})
}
```

Which can be run from the folder the file with the fuzz test case is in, as
follows:
```bash
~ go test -fuzz FuzzTestName
fuzz: elapsed: 0s, gathering baseline coverage: 0/192 completed
fuzz: elapsed: 0s, gathering baseline coverage: 192/192 completed, now fuzzing with 8 workers
fuzz: elapsed: 3s, execs: 325017 (108336/sec), new interesting: 11 (total: 202)
fuzz: elapsed: 6s, execs: 680218 (118402/sec), new interesting: 12 (total: 203)
fuzz: elapsed: 9s, execs: 1039901 (119895/sec), new interesting: 19 (total: 210)
fuzz: elapsed: 12s, execs: 1386684 (115594/sec), new interesting: 21 (total: 212)
PASS
ok      foo 12.692s
```

See the official [Go Fuzzing documentation][go-fuzz] for more details.

Running locally these tests is not very useful except for debugging purposes,
as in order to find bugs and vulnerabilities these require to run the for a
long time, and with a lot of different inputs. For this reason,
`crossplane/crossplane` and `crossplane/crossplane-runtime` were added to
OSS-Fuzz, which is exactly going to do that.

## OSS-Fuzz

OSS-Fuzz is a service run by Google that performs continuous fuzzing of open
source software. The fuzzers are run periodically, and any bugs that are found
are reported to the maintainers of the project. The fuzzers are also run
throughout the lifetime of the project, and any bugs that are found are
reported to the maintainers of the project.

See the [OSS-Fuzz documentation][oss-fuzz-arch] for more information on the
overall architecture.

## Relevant configurations

In order to properly hook up OSS-Fuzz to the Crossplane repositories, we had to
put in place a few configurations across different repositories:
- in `google/oss-fuzz` repository the [./projects/crossplane][oss-fuzz-folder]
- folder containing: a `project.yaml` defining all the details of the project,
- including the fuzzers to run, the maintainers, etc. a `Dockerfile` defining
- the build environment OSS-Fuzz will use to periodically build the fuzzers a
- `build.sh` script, which is copied into the image, that is just calling
- sequentially the two
    `test/fuzz/oss_fuzz_build.sh` scripts present in `crossplane/crossplane`
    and `crossplane/crossplane-runtime`.
- a `test/fuzz/oss_fuzz_build.sh` script in both
- [crossplane/crossplane][xp-fuzz_oss_build] and
    [crossplane/crossplane-runtime][xp-r-fuzz_oss_build], which are
    automatically finding the `Fuzz*` test cases in the respective repositories
    and building them as expected by OSS-Fuzz. See the header comment of each
    script for more details on their respective assumptions.
- a step in [crossplane/crossplane's ci pipeline][xp-ci] using [CIFuzz][CIFuzz]
- to run the fuzzers on every PR, 
    so that fuzzers are run also on PRs, leveraging the 30 day old/public
    regressions and corpora from OSS-Fuzz.
  - fuzzing is time intensive, we had to reduce the time the fuzzers are run
  - for, but it's still slowing down pipelines,
      for this reason we did not add it to `crossplane/crossplane-runtime` and
      we might move it to a separate non required workflow in the future in
      `crossplane/crossplane` too.


[CIFuzz]: https://google.github.io/oss-fuzz/getting-started/continuous-integration/
[fuzz-audit-report]: https://github.com/crossplane/crossplane/blob/master/security/ADA-fuzzing-audit-22.pdf
[go-fuzz]: https://go.dev/security/fuzz/
[oss-fuzz-arch]: https://google.github.io/oss-fuzz/architecture/
[oss-fuzz-folder]: https://github.com/google/oss-fuzz/tree/master/projects/crossplane
[oss-fuzz]: https://github.com/google/oss-fuzz
[xp-ci]: https://github.com/crossplane/crossplane/blob/master/.github/workflows/ci.yml
[xp-fuzz-tests]: https://github.com/search?q=repo%3Acrossplane%2Fcrossplane+%22func+Fuzz%22&type=code
[xp-fuzz_oss_build]: https://github.com/crossplane/crossplane/blob/master/test/fuzz/oss_fuzz_build.sh
[xp-r-fuzz_oss_build]: https://github.com/crossplane/crossplane-runtime/blob/master/test/fuzz/oss_fuzz_build.sh
