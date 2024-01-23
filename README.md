# PDP Client
A very light wrapper around OPA, with support for policies in git.


## Usage
### Library
This can be used as a go libary by running: `go get github.com/patrickfnielsen/pdp-client`

For an example usage see the included webserver under `/cmd/server/server.go`.

### Webserver
A fully functional webserver is included that when configured, will load the policies from git and make an endpoint available at `/api/v1/pdp/decision`.

The following envs are needed to run:
```
PDP_REPOSITORY
PDP_REPOSITORY_BRANCH
PDP_REPOSITORY_KEY
```

The following is optional, but usefull:
```
PDP_LOG_CONSOLE # enable console logging (default: true)
PDP_LOG_HTTP # enable http logging (default: false)
PDP_LOG_HTTP_SERVER # if http logging is enabled, specify the server to log to (default: "")
PDP_LOG_HTTP_SERVER_ENDPOINT # if http logging is enabled, specify the endpoint to log to (default: "/api/v1/decision/logs")
PDP_LOG_HTTP_SERVER_TOKEN # if http logging is enabled, specify the token to auth with (default: "")
PDP_LOG_HTTP_SERVER_TLS if http logging is enabled, enable / disable TLS (default: true)
```