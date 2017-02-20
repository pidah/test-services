# Design proposal for test-app 
There are two main use cases for which we need a pseudo application running in our kubernetes environments:

1. Check for cluster's state during provisioning and make sure integrations between components work.
2. Continuous cluster monitoring.

The first use case requires our app to respond to the queries (pull), and the second one suggests that we would like to report the state to external tools (push).

## Basic architecture

Microservice approach seems to be the best fit for this kind of application - it would closely mirror  other real-world applications that are run on the cluster and also would provide a number of built-in checks automatically. There would be a central "master" service, which polls information from all other services.

### Test microservice

Every microservice would serve a single test and report it's status via HTTP endpoint, `/status` . HTTP response code would be used to provide a high-level state:

* HTTP 200 OK for successful responses
* HTTP 503 Service unavailable for test failures

Response body would have a JSON encoded details about system state:

```
{
  "status": "ok",
  "description": "Dummy check",
  "message": "ping to bitesize-registry.default.. successful",
  "metrics": {
    "_comment": "This block is for future use to pass on custom test metrics"
  } 
}
```

There are 3 possible status levels:

1. ok
2. warning
3. error

In case of warning level messages, where it is needed to highlight that test is responding properly (within SLA) but is experiencing some minor problems (for example, latency is a bit higher than normal), test should respond with HTTP 200 OK status code and `status` set to `warning`. This level indicates that no action is needed, but it could help to diagnose underlying problems during the incident.

There would be two types of test microservices:

1. Short-running test. Microservice would trigger it's test during HTTP GET and report it's status synchronously.
2. Long-running test. Microservice would run it's test independent of HTTP requests periodically (configurable) and store test's state internally. HTTP GET would report this state. There should be a configurable state expiry (in microservice's settings), which would report HTTP 503 if state was not updated for expiry duration.

### Master service

Master service oversees other microservices and reports their status. It must have a configurable list of services to check (configuration file/environment variables):

1. Service name (e.g. `dummy`).
2. Service hostname (optional - default should be inherited from `SVC_DUMMY` kubernetes variable).
3. Check period (per service/global default).

This list also serves as an authoritative list of test services. It stores all  test states internally and initially, report them via two HTTP endpoints:
`/services` and `/service/{service_name}`. Response would be list of service statuses/single service status, with HTTP Response Code  corresponding with the overall system health.

* HTTP 200 OK - all services are healthy
* HTTP 503 Service unavailable - at least one service is responding with error (HTTP 503). This code is also used if one or more services cannot be reached (e.g. DNS errors, timeouts, connection refused problems).

It runs all checks independently (e.g. via go routines). After getting the response from the test-service, it should store service's state internally and push out metrics to statsd (configurable endpoint):

1. HTTP status code.
2. Request time.
3. (optional) list of metrics reported in `{service_name}` response's body.

This would allow us to integrate this service into external monitoring tools, like sysdig. How to configure statsd key prefix should be the subject of investigation and would depend on how it is handled in sysdig (could be configurable via config file/environment variable).

### Logging

Every service should log it's test results to `/dev/stdout`, complete with the timestamp, status and message, so that service's logs can be forwarded and stored in our logging systems. Master service should store error-level test service information and also any errors that occur during communication with test services (e.g. timeouts, DNS resolution problems, etc.).

### Alerting

Alerting is out of scope for this tool. All information that is needed for configuring alerts, is forwarded to statsd, where in it's own turn it can be forwarded to any monitoring solution we use (like sysdig). Alerts would be configured on monitoring solution's side. In cases where we need to get current status overview, we can use HTTP GET to poll information from master.

## Integration with deployment pipeline

It is desired that this tool would be built with deployment pipeline. However, there are multiple challenges around that - mostly about controlling multi-cluster deployments and package builds (we don't want to rebuilt all monitoring tools on all clusters we run on a single commit). Therefore, this topic is still open for discussion.

## List of test cases for test-services


The following test cases have been identified:

- A test to validate successful deployment via the jenkins deployment pipeline
- A test to validate inter-namespace communication (e.g communication between pulse and pulseauth)
- A test to validate stackstorm can deploy/provision a service via kubernetes 3rd Party resources; An example could be creating a simple AWS resource that can be easily torn down after tests complete e.g adding a route53 record or creating an empty s3 bucket
- A test to validate that the number of thirdpartyresources created == number of actual AWS resources created; The state should be captured so that we can easily determine a missed or failed event stackstorm event.
- A test to validate read/write operations to consul via stackstorm
- A test to validate read/write operations to vault via stackstorm
- A test to validate envconsul can pull and consume data from consul/vault
- A test to validate consumption of mongo service
- A test to validate comsumption of mysql service

This is not an exhaustive list and subject to review
