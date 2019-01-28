## Why zapr and logr
We decided to use `logr` as base interface for crossplane's logging system
after going through https://dave.cheney.net/2015/11/05/lets-talk-about-logging
and https://github.com/go-logr/logr/blob/master/README.md .
Dave's blog post in a nutshell explains why one doesn't need many logging levels but instead require `INFO`
and `DEBUG` levels with `ERROR` under `INFO` level. In development, both `INFO`
and `DEBUG` levels are enabled, but in production only `INFO` is enabled. `logr`
interface encourages any logging framework underneath to reflect Dave's ideas about logging.
[zapr](https://github.com/go-logr/zapr) is [logr](https://github.com/go-logr/logr) 's implementation of zap.
`zap/zap.go` uses Uber's [zap](https://github.com/uber-go/zap) with `zapr` implementation of `logr` interface.

## Using Logging library
`zap/zap.go` has 2 methods:
* `func NewLogger(env Environment, outputFormat OutputFormat) (logr.Logger, error)`
* `func NewLoggerTo(env Environment, outputFormat OutputFormat, outputWriter io.Writer,
   	errorOutputWriter io.Writer) (logr.Logger, error)`

When calling method have requirement of routing the logs to a `buffer` or some other
source they can use `NewLoggerTo`.

With calling any of the method, caller gets
reference to `logr.Logger` implementation. Caller can log `DEBUG` level by using
`V` method and setting parameter as `1` like below and is enabled only in dev environment
```
logger, _ :=  NewLogger(DevEnvironment, ConsoleFriendlyOutputFormat)
logger.WithName("crossplane-aws").WithValues("account_id", "1").V(1).Info("method1", "caller_id", "2")
```

produce result like this:
```
2019-01-27T19:01:23.704-0800  DEBUG  crossplane-aws  zap/zap_test.go:88  method  {"account_id": "1", "caller_id": "2"}`
```

Default without using `V` method or using `V(0)` is `INFO` level which is enabled in both prod and dev
environments. If caller intents to log an error, it is possible only with `INFO` level like this:
```
logger, _ :=  NewLogger(ProdEnvironment, JSONOutputFormat)
logger.WithName("crossplane-aws").WithValues("account_id", "1").Error(errors.New("error"), "method1", "caller_id", "2")
```
produce result like this:
```
{"level":"error","ts":1548645537800936000,"logger":"crossplane-aws","caller":"zap/zap_test.go:89","msg":"method1",
"account_id":"1","caller_id":"2","error":"error","stacktrace":"STACK_TRACE"}
```

## Known Issues in zapr
* Original `zap`'s logging levels could be acheived by calling `V` with negative int parameters
    as mentioned [here](https://github.com/go-logr/zapr/blob/master/zapr.go#L36). Its a hackaround to log in
    `zap`'s logging levels and callers are discouraged to use other logging levels.
