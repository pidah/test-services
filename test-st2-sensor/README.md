# test-tpr-s3

## Test St2 Sensor service.

This test service runs the `bitesize.check_st2_sensor action` via the stackstorm API every 5 minutes. The st2 action retrieves the st2 kubernetes sensors known by stackstorm (i.e stored in the st2 database) and compares the result against the actual sensor instances running on the stackstorm host. If there is a match, the state of the service will be saved as OK and return 200 status code. If the validation fails, the service returns a 503 Service Unavailable Status. The endpoint for this service can be hit with -> `curl test-st2-sensor.default.svc.cluster.local`
This service is polled by test-master https://github.com/pearsontechnology/test-master 



