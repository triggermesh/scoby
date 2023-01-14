# Logs

The controller rely on `controller-runtime` which uses the [logr](https://github.com/go-logr/logr) wrapper over [zap](https://github.com/uber-go/zap) logger.

- To enable development configuration use `--zap-devel`.
- Verbosity can be controlled by using `--zap-log-level <level>`.

## Verbosity

Scoby uses [logr](https://github.com/go-logr/logr) to write logs. When debugging we will use verbosity levels `0,1,5,10` in this way:

| V     | Description  |
|---    |---    |
| 0     | Always shown, equals to info level  |
| 1     | Equals to debug level |
| 2 - 5     | Use levels 2 - 5 for chatty debug logs |
| 6 - 10    | Use leves for spammy debug logs (eg. inside loops) |
