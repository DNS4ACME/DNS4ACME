#  Installing DNS4ACME on Kubernetes

DNS4ACME can run natively in a Kubernetes/OpenShift cluster. There are several ways you can achieve this, but each has its benefits and drawbacks. This document walks you through a few options.

!!! tip
    Installing DNS4ACME in Kubernetes doesn't mean you need to use the [Kubernetes backend](../configuration/backends/kubernetes.md). Inversely, you can also use the Kubernetes backend without running DNS4ACME inside a Kubernetes cluster.

!!! tip
    These examples contain no configuration see [the Configuration section](../configuration/index.md) for details on configuration.

---

## Deploying using the Helm chart (recommended)

We provide a [Helm chart](https://github.com/dns4acme/helm-chart) to deploy DNS4ACME. This is the easiest way to deploy DNS4ACME as it supports all installation methods outlined below.

---

## Deploying using a `Deployment` and a `LoadBalancer`

The most standard way to deploy DNS4ACME is to run it as a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/). In this case Kubernetes will run DNS4ACME inside a container and you will be able to expose the necessary TCP and UDP port 53 using a [Load Balancer](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer).

!!! note
    While this deployment method is the most straight-forward, your Kubernetes deployment may not have a Load Balancer available, or it may incur extra costs with your cloud provider.

You can create the Deployment resource as follows:

<details><summary>deployment.yaml</summary>
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dns4acme
  namespace: dns4acme # Customize this
spec:
  replicas: 1
  selector:
    matchLabels:
      - app.kubernetes.io/name: dns4acme
  template:
    metadata:
      labels:
        app.kubernetes.io/name: dns4acme
    spec:
      containers:
        - name: dns4acme
          image: ghcr.io/dns4acme/dns4acme
          env: [] # Customize this to include your desired configuration
          ports:
            - containerPort: 5353
              protocol: UDP
            - containerPort: 5353
              protocol: TCP
          livenessProbe:
            tcpSocket:
              port: 5353
```
</details>

You can then create the service like this:

<details><summary>loadbalancer.yaml</summary>
```yaml
apiVersion: v1
kind: Service
metadata:
  name: dns4acme
  namespace: dns4acme
  labels:
    app.kubernetes.io/name: dns4acme
spec:
  selector:
    app.kubernetes.io/name: dns4acme
  type: LoadBalancer
  ports:
    - port: 53
      protocol: UDP
      targetPort: 5353
    - port: 53
      protocol: TCP
      targetPort: 5353
```
</details>

Once deployed, the `status` field of the Service will provide you with the external IP address of the DNS server. you can then set up your DNS records to point to this IP address.

---

## Deploying using a `DaemonSet` and host networking

This method runs DNS4ACME as a DaemonSet on a specific set of nodes and uses privileged host network access to bind to port 53. The main tradeoff for this method is that the DNS4ACME pod runs on the host network and will need to bind to port 53, so DNS4ACME runs in somewhat of a privileged context. DNS4ACME also won't have access to the pod network.

This method only needs a DaemonSet:

<details><summary>daemonset.yaml</summary>
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: dns4acme
  namespace: dns4acme
  labels:
    app.kubernetes.io/name: dns4acme
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: dns4acme
  template:
    metadata:
      labels:
        app.kubernetes.io/name: dns4acme
    spec:
      # Schedule pods on control plane nodes only:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/control-plane
                    operator: Exists
      # Tolerate running on control plane nodes:
      tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      # Enable host networking:
      hostNetwork: true
      containers:
        - name: dns4acme
          image: ghcr.io/dns4acme/dns4acme
          env: # Customize this to include your desired configuration
          - name: DNS4ACME_LISTEN
            value: 0.0.0.0:53 # This is required
          securityContext:
            capabilities:
              add:
              - CAP_NET_BIND_SERVICE # This is needed to bind to port 53
          ports:
            - containerPort: 53
              protocol: UDP
            - containerPort: 53
              protocol: TCP
          livenessProbe:
            tcpSocket:
              port: 53
```
</details>

!!! warning
    In this mode DNS4ACME runs on the host's network and does not have direct access to the pod network. Depending on where your backend is located, you may need to provide *external* access parameters, such as `127.0.0.1:6443` for the Kubernetes API server. 

---

## Deploying using a `DaemonSet` and a `NodePort`

This mechanism starts the DNS4ACME daemon on every node (or optionally on the control plane nodes) and uses a Node Port to get the DNS traffic to DNS4ACME.

!!! warning
    The main drawback of this method is that you can't directly allocate port 53. In other words, you will need to create firewall rules externally to map port 53 to the desired NodePort.  

First, you need to create the DaemonSet to run DNS4ACME on your control plane nodes:

<details><summary>daemonset.yaml</summary>
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: dns4acme
  namespace: dns4acme
  labels:
    app.kubernetes.io/name: dns4acme
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: dns4acme
  template:
    metadata:
      labels:
        app.kubernetes.io/name: dns4acme
    spec:
      # Schedule pods on control plane nodes only:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/control-plane
                    operator: Exists
      # Tolerate running on control plane nodes:
      tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      containers:
        - name: dns4acme
          image: ghcr.io/dns4acme/dns4acme
          env: [] # Customize this to include your desired configuration
          ports:
            - containerPort: 5353
              protocol: UDP
            - containerPort: 5353
              protocol: TCP
          livenessProbe:
            tcpSocket:
              port: 5353
```
</details>

Now you can create your Node Port service:

<details><summary>nodeport.yaml</summary>
```yaml
apiVersion: v1
kind: Service
metadata:
  name: dns4acme
  namespace: dns4acme
  labels:
    app.kubernetes.io/name: dns4acme
spec:
  type: NodePort
  selector:
    app.kubernetes.io/name: dns4acme
  ports:
    - port: 53
      targetPort: 53535
      nodePort: 30053
```
</details>

!!! warning
    Typical Kubernetes clusters limit the Node Ports to a very high port range (default `30000`-`32767`). Usually you 
    cannot set a Node Port to port 53, and you will need to use an external means to get the traffic to DNS4ACME. 