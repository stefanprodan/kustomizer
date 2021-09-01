# Load test

Commands:

```shell
# count objects
kustomize build ./testdata/loadtest/ --load-restrictor=LoadRestrictionsNone | grep apiVersion | wc -l

kubectl apply -f apply -f ./testdata/loadtest/crd.yaml 

# kubectl dry-run
time kustomize build ./testdata/loadtest/ --load-restrictor=LoadRestrictionsNone | kubectl apply -f- --dry-run

# kubectl apply
time kustomize build ./testdata/loadtest/ --load-restrictor=LoadRestrictionsNone | kubectl apply -f-

# kubectl delete
time kustomize build ./testdata/loadtest/ --load-restrictor=LoadRestrictionsNone --reorder=none | kubectl delete -f-

# kustomizer dry-run + apply
time ./bin/kustomizer apply -k ./testdata/loadtest/ --inventory-name load-test

# kustomizer dry-run
time ./bin/kustomizer apply -k ./testdata/loadtest/ --inventory-name load-test

# kustomizer delete
time ./bin/kustomizer delete --inventory-name load-test
```

Env:

- Kubernetes v1.20.8-gke.2100
- kubectl v1.21.3
 
Results:

| Operation                                  | Time               | Objects            |
| ------------------------------------------ | ------------------ | ------------------ |
| kubectl    dry-run                         | 16.024s            | 110                |
| kustomizer dry-run                         | 15.258s            | 110                |
| kubectl    dry-run + apply                 | 35.770s            | 110                |
| kustomizer dry-run + apply                 | 25.575s            | 110                |
| kubectl    delete                          | 28.486s            | 110                |
| kustomizer delete                          | 21.507s            | 110                |


