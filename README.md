### kubernetes rocketmq operator

* operator-sdk version: v0.18.1
* kubernetes: v1.14+
* go version: v1.13+
* docker version: 17.03+
* rocketmq version: 4.4.0


### build 
```
operator-sdk build <IMAGE>:<tag>
```

### deploy operator

```
kubectl apply -f deploy/crds/middleware.io_rocketmqs_crd.yaml
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/role.yaml
kubectl apply -f deploy/service_account.yaml
kubectl apply -f deploy/role_binding.yaml
kubectl apply -f deploy/operator.yaml # replace image
```

### deploy rocketmq
```
kubectl apply -f deploy/crds/middleware.io_v1alpha1_rocketmq_cr.yaml
```