# Prometheus-AMQP

Send [Prometheus](https://prometheus.io/) metrics to an AMQP 1.0 queue

## Getting started
You need a prometheus installation, and some kind of AMQP 1.0 compatible message queue, e.g. RabbitMQ with AMQP1_0 plugin or Azure Service Bus / Azure Event Hub (that's what I am using it for).

### Docker / Kubernetes

You need the following informations about your AMQP endpoint:

 * URL: The target URL of the AMQP endpoint.
 This should start with `amqp://` or `amqps://`
 * Queue: The name of the queue, starting with a slash (`/`)

Build the container using the provided Dockerfile and set up a new deployment and service in Kubernetes:

```
apiVersion: extensions/v1beta1 
kind: Deployment
metadata:
  name: prometheusamqp
  namespace: monitoring
  labels:
    app: prometheusamqp
    component: amqp
spec:
  replicas: 1
  template:
    metadata:
      name: prometheusamqp
      labels:
        app: prometheusamqp
        component: amqp
    spec:
      containers:
      - name: prometheusamqp
        image: prometheus-amqp:latest
        env:
          - name: AMQP_ADDRESS
            value: amqps://<SASTokenName>:<SASTokenKey>@<namespace>.servicebus.windows.net
          - name: AMQP_QUEUE
            value: /<queuename>
        imagePullPolicy: Always
        ports:
        - name: write
          containerPort: 24282

---
apiVersion: v1
kind: Service
metadata:
  name: prometheusamqp
  namespace: monitoring
  labels:
    app: prometheusamqp
    component: amqp
  annotations:
    prometheus.io/scrape: 'true'
spec:
  type: NodePort
  ports:
    - port: 24282
      protocol: TCP
      name: write
  selector:
    app: prometheusamqp
    component: amqp

```

Of course you have to replace the relevat informations 

### Prometheus

Set up remote writing in prometheus by adding this to your prometheus configuration:
```
remote_write:
  - url: "http://prometheusamqp.monitoring:24282/write"
```

## Example

If everything is set up it you should start to see metric messages on the queue as JSON, e.g. for Azure AKS:
```
{
	"metric": {
		"__name__": "container_tasks_state",
		"agentpool": "agentpool",
		"beta_kubernetes_io_arch": "amd64",
		"beta_kubernetes_io_instance_type": "Standard_DS1_v2",
		"beta_kubernetes_io_os": "linux",
		"failure_domain_beta_kubernetes_io_region": "westeurope",
		"failure_domain_beta_kubernetes_io_zone": "0",
		"id": "/system.slice/setvtrgb.service",
		"instance": "aks-agentpool-12345678",
		"job": "kubernetes-cadvisor",
		"kubernetes_azure_com_cluster": "MC_rg_aks_westeurope",
		"kubernetes_io_hostname": "aks-agentpool-12345678",
		"kubernetes_io_role": "agent",
		"state": "running",
		"storageprofile": "managed",
		"storagetier": "Premium_LRS"
	},
	"value": [1530103567.683, "0"]
}
```

## Filtering

Because Prometheus create A LOT of metrics, you might overwhelm your processes that consume the messages on the queue. 
For this you can use a simple label filter, given by the parameter `--filter-file`.

### Filter format

Within the filter file you can write one filter per line in the format
`[LABELNAME] [OPERATION] [CONTENT]`, e.g. `__name__ SI nginx_http_requests_total`.

Valid operations are
Operation|Action|Case-Sensitive
---------|------|--------------
SI|Label name Starts with|no
SC|Label name Starts with|yes
EI|Label name Equals|no
EC|Label name Equals|yes
CI|Label name Contains|no
CC|Label name Contains|yes

The example from above would therefore keep all metrics, that have a value starting with `nginx_http_requests_total` on a label named `__name__`.
Other examples might be `app EC typo3` to keep all metrics having a label named `app` which has a value of exactly `typo3`.

Warning: If you use the filterfile, all metrics not matching a filter will be discarded silently! 
If the file is empty, all metrics will be returned.

## Log only

If you set the parameter `--log-only`, no metrics will be sent to the queue but instead being logged to the console. This is useful to set up filters before sending thousands of metrics to the queue.

## License

This project is licensed under the MIT License - see the LICENSE file for details

## Acknowledgments

This tool is heavily inspired by the [prometheus remote writer](https://github.com/prometheus/prometheus/tree/release-2.3/documentation/examples/remote_storage/remote_storage_adapter).

Using [AMQP 1.0 client library for Go](https://github.com/vcabbage/amqp) by vcabbage